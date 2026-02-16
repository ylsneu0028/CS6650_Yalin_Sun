func validateProduct(pathID int32, p api.Product) *api.ErrorResp {
  if pathID < 1 {
    return &api.ErrorResp{Error: "INVALID_INPUT", Message: "Invalid productId", Details: "productId must be >= 1"}
  }
  if p.ProductID < 1 {
    return &api.ErrorResp{Error: "INVALID_INPUT", Message: "Invalid product_id", Details: "product_id must be >= 1"}
  }
  if p.ProductID != pathID {
    return &api.ErrorResp{Error: "INVALID_INPUT", Message: "product_id mismatch", Details: "product_id must match path productId"}
  }
  if len(p.SKU) < 1 || len(p.SKU) > 100 {
    return &api.ErrorResp{Error: "INVALID_INPUT", Message: "Invalid sku", Details: "sku length must be 1-100"}
  }
  if len(p.Manufacturer) < 1 || len(p.Manufacturer) > 200 {
    return &api.ErrorResp{Error: "INVALID_INPUT", Message: "Invalid manufacturer", Details: "manufacturer length must be 1-200"}
  }
  if p.CategoryID < 1 {
    return &api.ErrorResp{Error: "INVALID_INPUT", Message: "Invalid category_id", Details: "category_id must be >= 1"}
  }
  if p.Weight < 0 {
    return &api.ErrorResp{Error: "INVALID_INPUT", Message: "Invalid weight", Details: "weight must be >= 0"}
  }
  if p.SomeOtherID < 1 {
    return &api.ErrorResp{Error: "INVALID_INPUT", Message: "Invalid some_other_id", Details: "some_other_id must be >= 1"}
  }
  return nil
}

