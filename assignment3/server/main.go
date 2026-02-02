package main

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

type album struct {
	ID     string  `json:"id"`
	Title  string  `json:"title"`
	Artist string  `json:"artist"`
	Price  float64 `json:"price"`
}

var (
	albums = []album{
		{ID: "1", Title: "Blue Train", Artist: "John Coltrane", Price: 56.99},
		{ID: "2", Title: "Jeru", Artist: "Gerry Mulligan", Price: 17.99},
		{ID: "3", Title: "Sarah Vaughan and Clifford Brown", Artist: "Sarah Vaughan", Price: 39.99},
	}
	albumsMu sync.RWMutex
)

func main() {
	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/albums", getAlbums)
	router.GET("/albums/:id", getAlbumByID)
	router.POST("/albums", postAlbums)
	router.Run(":8080")
}

func getAlbums(c *gin.Context) {
	albumsMu.RLock()
	defer albumsMu.RUnlock()

	// copy to avoid holding lock while JSON encoding (optional but good practice)
	out := make([]album, len(albums))
	copy(out, albums)

	c.JSON(http.StatusOK, out)
}

func postAlbums(c *gin.Context) {
	var newAlbum album
	if err := c.BindJSON(&newAlbum); err != nil {
		return
	}

	albumsMu.Lock()
	albums = append(albums, newAlbum)
	albumsMu.Unlock()

	c.JSON(http.StatusCreated, newAlbum)
}

func getAlbumByID(c *gin.Context) {
	id := c.Param("id")

	albumsMu.RLock()
	defer albumsMu.RUnlock()

	for _, a := range albums {
		if a.ID == id {
			c.JSON(http.StatusOK, a)
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"message": "album not found"})
}
