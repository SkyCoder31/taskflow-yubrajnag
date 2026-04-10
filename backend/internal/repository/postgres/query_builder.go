package postgres

import (
	"fmt"
	"strings"
)

// QueryBuilder constructs parameterized SQL with dynamic WHERE clauses.
// Uses %s as placeholder, auto-replaced with $N positional parameters.
type QueryBuilder struct {
	selectBase string
	countBase  string
	conditions []string
	args       []any
	orderBy    string
	page       int
	limit      int
	paramIndex int
}

func NewQueryBuilder(selectBase, countBase string) *QueryBuilder {
	return &QueryBuilder{
		selectBase: selectBase,
		countBase:  countBase,
		paramIndex: 1,
	}
}

func (qb *QueryBuilder) Where(clause string, arg any) *QueryBuilder {
	parameterized := strings.Replace(clause, "%s", fmt.Sprintf("$%d", qb.paramIndex), 1)
	qb.conditions = append(qb.conditions, parameterized)
	qb.args = append(qb.args, arg)
	qb.paramIndex++
	return qb
}

func (qb *QueryBuilder) WhereIf(condition bool, clause string, arg any) *QueryBuilder {
	if condition {
		return qb.Where(clause, arg)
	}
	return qb
}

func (qb *QueryBuilder) OrderBy(clause string) *QueryBuilder {
	qb.orderBy = clause
	return qb
}

func (qb *QueryBuilder) Paginate(page, limit int) *QueryBuilder {
	if page > 0 && limit > 0 {
		qb.page = page
		qb.limit = limit
	}
	return qb
}

// Build returns data query + count query with their respective args.
func (qb *QueryBuilder) Build() (query string, queryArgs []any, countQuery string, countArgs []any) {
	whereClause := ""
	if len(qb.conditions) > 0 {
		whereClause = " WHERE " + strings.Join(qb.conditions, " AND ")
	}

	// Count query: base + WHERE (no ORDER BY, no LIMIT)
	var countSB strings.Builder
	countSB.WriteString(qb.countBase)
	countSB.WriteString(whereClause)
	countQuery = countSB.String()
	countArgs = make([]any, len(qb.args))
	copy(countArgs, qb.args)

	// Data query: base + WHERE + ORDER BY + LIMIT/OFFSET
	var sb strings.Builder
	sb.WriteString(qb.selectBase)
	sb.WriteString(whereClause)

	if qb.orderBy != "" {
		sb.WriteString(" ORDER BY ")
		sb.WriteString(qb.orderBy)
	}

	queryArgs = make([]any, len(qb.args))
	copy(queryArgs, qb.args)

	if qb.limit > 0 {
		offset := (qb.page - 1) * qb.limit
		sb.WriteString(fmt.Sprintf(" LIMIT $%d OFFSET $%d", qb.paramIndex, qb.paramIndex+1))
		queryArgs = append(queryArgs, qb.limit, offset)
	}

	query = sb.String()
	return
}
