package repository

import (
	"backend/internal/model"
	"context"
)

type ProductRepository struct {
	db DBTX
}

func NewProductRepository(db DBTX) *ProductRepository {
	return &ProductRepository{db: db}
}

// 商品一覧を指定した部分を取得します
func (r *ProductRepository) ListProducts(ctx context.Context, userID int, req model.ListRequest) ([]model.Product, int, error) {
    var products []model.Product

    // --- Build WHERE ---
    whereClause := ""
    args := []interface{}{}
    if req.Search != "" {
        whereClause = " WHERE (name LIKE ? OR description LIKE ?)"
        pattern := "%" + req.Search + "%"
        args = append(args, pattern, pattern)
    }

    // --- Build ORDER ---
    orderClause := " ORDER BY " + req.SortField + " " + req.SortOrder + ", product_id ASC"

    // --- Main query with LIMIT/OFFSET ---
    query := `
        SELECT product_id, name, value, weight, image, description
        FROM products
    ` + whereClause + orderClause + " LIMIT ? OFFSET ?"

    argsForSelect := append(append([]interface{}{}, args...), req.PageSize, req.Offset)

    query = r.db.Rebind(query)

    err := r.db.SelectContext(ctx, &products, query, argsForSelect...)
    if err != nil {
        return nil, 0, err
    }

    // --- COUNT(*) ---
    countQuery := "SELECT COUNT(*) FROM products" + whereClause
    countQuery = r.db.Rebind(countQuery)

    var total int
    err = r.db.GetContext(ctx, &total, countQuery, args...)
    if err != nil {
        return nil, 0, err
    }

    return products, total, nil
}
