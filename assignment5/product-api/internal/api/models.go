package api

type Product struct {
  ProductID    int32  `json:"product_id"`
  SKU          string `json:"sku"`
  Manufacturer string `json:"manufacturer"`
  CategoryID   int32  `json:"category_id"`
  Weight       int32  `json:"weight"`
  SomeOtherID  int32  `json:"some_other_id"`
}

type ErrorResp struct {
  Error   string `json:"error"`
  Message string `json:"message"`
  Details string `json:"details,omitempty"`
}
