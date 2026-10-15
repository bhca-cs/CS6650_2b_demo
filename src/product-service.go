package main

import (
	"errors"
	"net/http"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
)

// ===================== DATA MODEL =====================

// Product mirrors the "Product" schema in api.yaml.
// All 6 fields are required per the spec.
type Product struct {
	ProductID    int    `json:"product_id" binding:"required,min=1"`
	SKU          string `json:"sku" binding:"required,min=1,max=100"` // "sku" is "stock keeping unit"
	Manufacturer string `json:"manufacturer" binding:"required,min=1,max=200"`
	CategoryID   int    `json:"category_id" binding:"required,min=1"`
	Weight       int    `json:"weight" binding:"min=0"`
	SomeOtherID  int    `json:"some_other_id" binding:"required,min=1"`
}

// ErrorResponse mirrors the "Error" schema in api.yaml.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// ===================== IN-MEMORY STORAGE =====================

// ProductStore holds products in memory, protected by an RWMutex.
// Reads (GET) will likely dominate over writes (POST) in a real
// e-commerce system, customers browse products far more often than
// product details get updated, so RWMutex lets concurrent reads
// proceed without blocking each other.
type ProductStore struct {
	mu       sync.RWMutex
	products map[int]Product
}

func NewProductStore() *ProductStore {
	return &ProductStore{
		products: make(map[int]Product),
	}
}

func (s *ProductStore) Get(id int) (Product, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.products[id]
	return p, ok
}

func (s *ProductStore) Exists(id int) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.products[id]
	return ok
}

func (s *ProductStore) Set(id int, p Product) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.products[id] = p
}

var store = NewProductStore()

// errInvalidID is returned when a path parameter fails the
// "integer, minimum: 1" constraint from api.yaml.
var errInvalidID = errors.New("productId must be a positive integer")

// ===================== SETUP =====================

func main() {
	router := gin.Default()

	// Pre-seed a couple of products so GET has something to find
	// immediately, and so POST /details has existing products to update -
	// matching Lucia's clarification that POST requires the product to
	// already exist; it does not create one.
	store.Set(1, Product{
		ProductID:    1,
		SKU:          "SEED-0001",
		Manufacturer: "Seed Co.",
		CategoryID:   1,
		Weight:       100,
		SomeOtherID:  1,
	})
	store.Set(2, Product{
		ProductID:    2,
		SKU:          "SEED-0002",
		Manufacturer: "Seed Co.",
		CategoryID:   2,
		Weight:       200,
		SomeOtherID:  2,
	})

	router.GET("/products/:productId", getProduct)
	router.POST("/products/:productId/details", addProductDetails)

	router.Run(":8080")
}

// ===================== HANDLERS =====================

// getProduct implements GET /products/{productId} exactly as specified
// in api.yaml: 200 + Product on success, 404 + Error if not found,
// 500 + Error on unexpected failure.
func getProduct(c *gin.Context) {
	productID, err := parseProductID(c)
	if err != nil {
		// A malformed/invalid ID isn't explicitly listed as its own
		// response code for GET in the spec, so we treat it the same as
		// "not found" - from the client's perspective, an invalid ID can
		// never resolve to a real product either way.
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "PRODUCT_NOT_FOUND",
			Message: "Product not found",
			Details: err.Error(),
		})
		return
	}

	product, found := store.Get(productID)
	if !found {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "PRODUCT_NOT_FOUND",
			Message: "Product not found",
			Details: "No product exists with the given ID",
		})
		return
	}

	c.JSON(http.StatusOK, product)
}

// addProductDetails implements POST /products/{productId}/details exactly
// as specified: 204 on success (no content), 400 for invalid input,
// 404 if the product doesn't already exist, 500 on unexpected failure.
func addProductDetails(c *gin.Context) {
	productID, err := parseProductID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "INVALID_INPUT",
			Message: "Invalid product ID",
			Details: err.Error(),
		})
		return
	}

	// Per Lucia's clarification: this endpoint is about the DETAILS,
	// not the product itself - the product must already exist, or we 404.
	// "The POST is about the detail, not the product... it overreaches
	// to create a new product when the endpoint is about details."
	if !store.Exists(productID) {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "PRODUCT_NOT_FOUND",
			Message: "Product not found",
			Details: "Cannot add details to a product that does not exist",
		})
		return
	}

	var body Product
	// ShouldBindJSON validates the request body against the struct tags
	// above (required, min, max) - this is where Gin enforces most of
	// the Product schema's constraints automatically, and also catches
	// malformed JSON itself.
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "INVALID_INPUT",
			Message: "The provided input data is invalid",
			Details: err.Error(),
		})
		return
	}

	// Extra validation Gin's binding tags can't express on their own:
	// the path's productId and the body's product_id must refer to the
	// SAME product. A mismatch is a client error (ambiguous intent),
	// not something we should silently resolve one way or the other.
	if body.ProductID != productID {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "INVALID_INPUT",
			Message: "product_id in request body does not match productId in URL path",
			Details: "The product being updated must match the URL",
		})
		return
	}

	store.Set(productID, body)
	c.Status(http.StatusNoContent)
}

// ===================== HELPERS =====================

// parseProductID extracts and validates the :productId path parameter,
// enforcing the spec's "integer, minimum: 1" constraint.
func parseProductID(c *gin.Context) (int, error) {
	idStr := c.Param("productId")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return 0, errInvalidID
	}
	if id < 1 {
		return 0, errInvalidID
	}
	return id, nil
}
