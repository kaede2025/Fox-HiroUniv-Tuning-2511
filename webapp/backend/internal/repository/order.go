package repository

import (
	"backend/internal/model"
	"context"
	"database/sql"
	"fmt"
	// "sort"
	"strings"

	"github.com/jmoiron/sqlx"
)

type OrderRepository struct {
	db DBTX
}

func NewOrderRepository(db DBTX) *OrderRepository {
	return &OrderRepository{db: db}
}

// 注文を作成し、生成された注文IDを返す
func (r *OrderRepository) Create(ctx context.Context, order *model.Order) (string, error) {
	query := `INSERT INTO orders (user_id, product_id, shipped_status, created_at) VALUES (?, ?, 'shipping', NOW())`
	result, err := r.db.ExecContext(ctx, query, order.UserID, order.ProductID)
	if err != nil {
		return "", err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", id), nil
}

// 複数の注文IDのステータスを一括で更新
// 主に配送ロボットが注文を引き受けた際に一括更新をするために使用
func (r *OrderRepository) UpdateStatuses(ctx context.Context, orderIDs []int64, newStatus string) error {
	if len(orderIDs) == 0 {
		return nil
	}
	query, args, err := sqlx.In("UPDATE orders SET shipped_status = ? WHERE order_id IN (?)", newStatus, orderIDs)
	if err != nil {
		return err
	}
	query = r.db.Rebind(query)
	_, err = r.db.ExecContext(ctx, query, args...)
	return err
}

// 配送中(shipped_status:shipping)の注文一覧を取得
func (r *OrderRepository) GetShippingOrders(ctx context.Context) ([]model.Order, error) {
	var orders []model.Order
	query := `
        SELECT
            o.order_id,
            p.weight,
            p.value
        FROM orders o
        JOIN products p ON o.product_id = p.product_id
        WHERE o.shipped_status = 'shipping'
    `
	err := r.db.SelectContext(ctx, &orders, query)
	return orders, err
}

// 注文履歴一覧を取得

func (r *OrderRepository) ListOrders(ctx context.Context, userID int, req model.ListRequest) ([]model.Order, int, error) {

    // --- 构建 WHERE ---
    whereParts := []string{"o.user_id = ?"}
    args := []interface{}{userID}

    if req.Search != "" {
        if req.Type == "prefix" {
            whereParts = append(whereParts, "p.name LIKE ?")
            args = append(args, req.Search+"%")
        } else {
            whereParts = append(whereParts, "p.name LIKE ?")
            args = append(args, "%"+req.Search+"%")
        }
    }

    whereClause := "WHERE " + strings.Join(whereParts, " AND ")

    // --- 排序字段转换（防 SQL 注入） ---
    validSortFields := map[string]string{
        "order_id":       "o.order_id",
        "product_name":   "p.name",
        "created_at":     "o.created_at",
        "arrived_at":     "o.arrived_at",
        "shipped_status": "o.shipped_status",
    }

    sortField, ok := validSortFields[req.SortField]
    if !ok {
        sortField = "o.order_id"
    }

    sortOrder := strings.ToUpper(req.SortOrder)
    if sortOrder != "ASC" && sortOrder != "DESC" {
        sortOrder = "ASC"
    }

    orderClause := "ORDER BY " + sortField + " " + sortOrder + ", o.order_id ASC"

    // --- LIMIT / OFFSET ---
    limitClause := "LIMIT ? OFFSET ?"
    argsWithPaging := append(append([]interface{}{}, args...), req.PageSize, req.Offset)

    // --- 主查询 ---
    query := `
        SELECT 
            o.order_id,
            o.product_id,
            p.name AS product_name,
            o.shipped_status,
            o.created_at,
            o.arrived_at
        FROM orders o
        JOIN products p ON o.product_id = p.product_id
        ` + whereClause + `
        ` + orderClause + `
        ` + limitClause

    query = r.db.Rebind(query)

    var rows []struct {
        OrderID       int64         `db:"order_id"`
        ProductID     int           `db:"product_id"`
        ProductName   string        `db:"product_name"`
        ShippedStatus string        `db:"shipped_status"`
        CreatedAt     sql.NullTime  `db:"created_at"`
        ArrivedAt     sql.NullTime  `db:"arrived_at"`
    }

    if err := r.db.SelectContext(ctx, &rows, query, argsWithPaging...); err != nil {
        return nil, 0, err
    }

    // --- COUNT(*) 获取总数 ---
    countQuery := `
        SELECT COUNT(*)
        FROM orders o
        JOIN products p ON o.product_id = p.product_id
    ` + whereClause
    countQuery = r.db.Rebind(countQuery)

    var total int
    if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
        return nil, 0, err
    }

    // --- 转换成 model.Order ---
    orders := make([]model.Order, 0, len(rows))
    for _, r2 := range rows {
        orders = append(orders, model.Order{
            OrderID:       r2.OrderID,
            ProductID:     r2.ProductID,
            ProductName:   r2.ProductName,
            ShippedStatus: r2.ShippedStatus,
            CreatedAt:     r2.CreatedAt.Time,
            ArrivedAt:     r2.ArrivedAt,
        })
    }

    return orders, total, nil
}