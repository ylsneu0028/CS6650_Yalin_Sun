package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"golang.org/x/sync/semaphore"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/cs6650/album-store/internal/models"
	"github.com/cs6650/album-store/internal/store"
)

// OpenAPI: photo file up to 200 MiB. Whole HTTP body is larger (boundaries, headers, CRLF).
const maxPhotoFileBytes = 200 << 20
// Generous headroom: some proxies/clients add extra CRLF or duplicate headers; avoid false 400/EOF.
const maxMultipartBodyBytes = maxPhotoFileBytes + (128 << 20)

type Handler struct {
	Store        *store.Store
	Region       string
	UploadSlots  *semaphore.Weighted // limits concurrent large body reads + temp files (avoids OOM under S15)
}

func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, models.HealthResponse{Status: "ok"})
}

func (h *Handler) PutAlbum(w http.ResponseWriter, r *http.Request) {
	albumID := chi.URLParam(r, "album_id")
	if _, err := uuid.Parse(albumID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid album_id")
		return
	}
	var req models.AlbumRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.AlbumID != albumID {
		writeError(w, http.StatusBadRequest, "album_id mismatch")
		return
	}
	a := &models.Album{
		AlbumID:     req.AlbumID,
		Title:       req.Title,
		Description: req.Description,
		Owner:       req.Owner,
	}
	_, getErr := h.Store.GetAlbum(r.Context(), albumID)
	status := http.StatusCreated
	if getErr == nil {
		status = http.StatusOK
	} else if !errors.Is(getErr, store.ErrNotFound) {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	if err := h.Store.PutAlbum(r.Context(), a); err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, status, a)
}

func (h *Handler) GetAlbum(w http.ResponseWriter, r *http.Request) {
	albumID := chi.URLParam(r, "album_id")
	a, err := h.Store.GetAlbum(r.Context(), albumID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func (h *Handler) ListAlbums(w http.ResponseWriter, r *http.Request) {
	list, err := h.Store.ListAlbums(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	if list == nil {
		list = []models.Album{}
	}
	writeJSON(w, http.StatusOK, list)
}

func writeUploadClientError(w http.ResponseWriter, err error) {
	var mbe *http.MaxBytesError
	if errors.As(err, &mbe) {
		writeError(w, http.StatusRequestEntityTooLarge, "request too large")
		return
	}
	// multipart may wrap the underlying reader error; substring covers edge cases.
	if err != nil && strings.Contains(err.Error(), "too large") {
		writeError(w, http.StatusRequestEntityTooLarge, "request too large")
		return
	}
	writeError(w, http.StatusBadRequest, "missing or malformed photo field")
}

func (h *Handler) UploadPhoto(w http.ResponseWriter, r *http.Request) {
	albumID := chi.URLParam(r, "album_id")
	if _, err := uuid.Parse(albumID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid album_id")
		return
	}

	if h.UploadSlots != nil {
		if err := h.UploadSlots.Acquire(r.Context(), 1); err != nil {
			writeError(w, http.StatusServiceUnavailable, "unavailable")
			return
		}
		defer h.UploadSlots.Release(1)
	}

	mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || !strings.HasPrefix(mediaType, "multipart/form-data") {
		writeError(w, http.StatusBadRequest, "missing or malformed photo field")
		return
	}
	boundary := params["boundary"]
	if boundary == "" {
		writeError(w, http.StatusBadRequest, "missing or malformed photo field")
		return
	}

	body := http.MaxBytesReader(w, r.Body, maxMultipartBodyBytes+1)
	mr := multipart.NewReader(body, boundary)

	var photoPart *multipart.Part
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			writeUploadClientError(w, err)
			return
		}
		if part.FormName() == "photo" {
			photoPart = part
			break
		}
		_, _ = io.Copy(io.Discard, part)
		_ = part.Close()
	}
	if photoPart == nil {
		writeError(w, http.StatusBadRequest, "photo field not found in multipart body")
		return
	}

	seq, err := h.Store.NextPhotoSeq(r.Context(), albumID)
	if err != nil {
		_, _ = io.Copy(io.Discard, photoPart)
		_ = photoPart.Close()
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	photoID := uuid.NewString()

	if err := h.Store.PutPhoto(r.Context(), &store.PhotoItem{
		AlbumID: albumID,
		PhotoID: photoID,
		Seq:     seq,
		Status:  "processing",
	}); err != nil {
		_, _ = io.Copy(io.Discard, photoPart)
		_ = photoPart.Close()
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	contentType := photoPart.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	tmp, err := os.CreateTemp("", "album-photo-*")
	if err != nil {
		_ = photoPart.Close()
		_ = h.Store.UpdatePhotoFailed(context.Background(), albumID, photoID)
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	tmpPath := tmp.Name()
	_, err = io.Copy(tmp, photoPart)
	_ = photoPart.Close()
	_ = tmp.Close()
	if err != nil {
		_ = os.Remove(tmpPath)
		_ = h.Store.UpdatePhotoFailed(context.Background(), albumID, photoID)
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			writeError(w, http.StatusRequestEntityTooLarge, "request too large")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		_, _ = io.Copy(io.Discard, p)
		_ = p.Close()
	}

	writeJSON(w, http.StatusAccepted, models.PhotoAccepted{
		PhotoID: photoID,
		Seq:     seq,
		Status:  "processing",
	})

	region := h.Region
	bucket := h.Store.BucketName()
	key := store.PhotoObjectKey(albumID, photoID)

	go h.processPhotoUpload(context.Background(), albumID, photoID, tmpPath, contentType, key, bucket, region)
}

func (h *Handler) processPhotoUpload(
	ctx context.Context,
	albumID, photoID, tmpPath string,
	contentType, key, bucket, region string,
) {
	defer func() { _ = os.Remove(tmpPath) }()

	f, err := os.Open(tmpPath)
	if err != nil {
		_ = h.Store.UpdatePhotoFailed(ctx, albumID, photoID)
		return
	}
	defer f.Close()

	if err := h.Store.UploadObject(ctx, key, f, contentType); err != nil {
		_ = h.Store.UpdatePhotoFailed(ctx, albumID, photoID)
		return
	}

	publicURL := store.PublicObjectURL(bucket, region, key)
	if err := h.Store.UpdatePhotoCompleted(ctx, albumID, photoID, key, publicURL); err != nil {
		_ = h.Store.DeleteObject(ctx, key)
		var cfe *types.ConditionalCheckFailedException
		if errors.As(err, &cfe) {
			return
		}
		_ = h.Store.UpdatePhotoFailed(ctx, albumID, photoID)
	}
}

func (h *Handler) GetPhoto(w http.ResponseWriter, r *http.Request) {
	albumID := chi.URLParam(r, "album_id")
	photoID := chi.URLParam(r, "photo_id")
	ps, err := h.Store.GetPhoto(r.Context(), albumID, photoID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, ps)
}

func (h *Handler) DeletePhoto(w http.ResponseWriter, r *http.Request) {
	albumID := chi.URLParam(r, "album_id")
	photoID := chi.URLParam(r, "photo_id")

	item, err := h.Store.FetchPhotoItem(r.Context(), albumID, photoID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		// Contract: avoid 5xx on DELETE
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if item.S3Key != "" {
		_ = h.Store.DeleteObject(r.Context(), item.S3Key)
	} else if item.Status == "completed" && item.PublicURL != "" {
		// Derive key from URL if older records lack s3_key
		if k := keyFromPublicURL(item.PublicURL, h.Store.BucketName()); k != "" {
			_ = h.Store.DeleteObject(r.Context(), k)
		}
	}

	if err := h.Store.DeletePhotoRecord(r.Context(), albumID, photoID); err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func keyFromPublicURL(publicURL, bucket string) string {
	u, err := url.Parse(publicURL)
	if err != nil {
		return ""
	}
	if !strings.HasPrefix(u.Host, bucket+".") || !strings.Contains(u.Host, ".s3.") {
		return ""
	}
	return strings.TrimPrefix(u.Path, "/")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, models.ErrorBody{Error: msg})
}
