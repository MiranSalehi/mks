package mks

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/goravel/framework/contracts/database/db"
	"github.com/goravel/framework/contracts/database/orm"
)

// policyQuery implements the predicate-only Policy.Scope contract. Every predicate
// returns this wrapper so Scopes is eager even after another predicate. There is
// deliberately no embedded orm.Query: promoting a method can escape the wrapper
// and silently discard deferred scopes or other authorization constraints.
type policyQuery struct {
	query orm.Query
	group *policyQueryGroup
}

type policyQueryGroup struct{ query orm.Query }

var _ orm.Query = policyQuery{}

// policyWhereGroup is also used for nested Where/OrWhere callbacks. Goravel's
// buildSubquery accepts only its concrete query and builds only WHERE conditions,
// so each callback must return the underlying query after all scopes expand.
func policyWhereGroup(query orm.Query, predicate func(orm.Query) orm.Query) orm.Query {
	q := policyQuery{query: query, group: &policyQueryGroup{query: query}}
	return q.result(predicate(q)).query
}

func (q policyQuery) result(result orm.Query) policyQuery {
	wrapped, ok := result.(policyQuery)
	if !ok || wrapped.group != q.group {
		panic("mks: Policy.Scope and its callbacks must return the supplied predicate query")
	}
	return wrapped
}

func (q policyQuery) next(query orm.Query) orm.Query {
	q.query = query
	return q
}

func policyPredicate(query any) any {
	if predicate, ok := query.(func(orm.Query) orm.Query); ok {
		return func(group orm.Query) orm.Query {
			return policyWhereGroup(group, predicate)
		}
	}
	return query
}

func (q policyQuery) Scopes(scopes ...func(orm.Query) orm.Query) orm.Query {
	for _, scope := range scopes {
		q = q.result(scope(q))
	}
	return q
}

func (q policyQuery) Where(query any, args ...any) orm.Query {
	return q.next(q.query.Where(policyPredicate(query), args...))
}

func (q policyQuery) OrWhere(query any, args ...any) orm.Query {
	return q.next(q.query.OrWhere(policyPredicate(query), args...))
}

// Unsupported operations panic before they can execute SQL or escape the wrapper.
// orm.Query provides no way to attach an error to a grouped predicate; silently
// ignoring these operations could broaden authorization (e.g. a dropped HAVING).
func unsupportedPolicyQuery(operation string) error {
	return fmt.Errorf("mks: Policy.Scope only supports WHERE predicates and predicate Scopes; %s is unsupported", operation)
}

func (q policyQuery) Association(association string) orm.Association {
	panic(unsupportedPolicyQuery("Association"))
}

func (q policyQuery) Begin() (orm.Query, error) {
	panic(unsupportedPolicyQuery("Begin"))
}

func (q policyQuery) BeginTransaction() (orm.Query, error) {
	panic(unsupportedPolicyQuery("BeginTransaction"))
}

func (q policyQuery) Commit() error {
	panic(unsupportedPolicyQuery("Commit"))
}

func (q policyQuery) Context() context.Context {
	panic(unsupportedPolicyQuery("Context"))
}

func (q policyQuery) Count() (int64, error) {
	panic(unsupportedPolicyQuery("Count"))
}

func (q policyQuery) Create(value any) error {
	panic(unsupportedPolicyQuery("Create"))
}

func (q policyQuery) Cursor() chan db.Row {
	panic(unsupportedPolicyQuery("Cursor"))
}

func (q policyQuery) DB() (*sql.DB, error) {
	panic(unsupportedPolicyQuery("DB"))
}

func (q policyQuery) Delete(value ...any) (*db.Result, error) {
	panic(unsupportedPolicyQuery("Delete"))
}

func (q policyQuery) Distinct(columns ...string) orm.Query {
	panic(unsupportedPolicyQuery("Distinct"))
}

func (q policyQuery) Driver() string {
	panic(unsupportedPolicyQuery("Driver"))
}

func (q policyQuery) Exec(sql string, values ...any) (*db.Result, error) {
	panic(unsupportedPolicyQuery("Exec"))
}

func (q policyQuery) Exists() (bool, error) {
	panic(unsupportedPolicyQuery("Exists"))
}

func (q policyQuery) Find(dest any, conds ...any) error {
	panic(unsupportedPolicyQuery("Find"))
}

func (q policyQuery) FindOrFail(dest any, conds ...any) error {
	panic(unsupportedPolicyQuery("FindOrFail"))
}

func (q policyQuery) First(dest any) error {
	panic(unsupportedPolicyQuery("First"))
}

func (q policyQuery) FirstOr(dest any, callback func() error) error {
	panic(unsupportedPolicyQuery("FirstOr"))
}

func (q policyQuery) FirstOrCreate(dest any, conds ...any) error {
	panic(unsupportedPolicyQuery("FirstOrCreate"))
}

func (q policyQuery) FirstOrFail(dest any) error {
	panic(unsupportedPolicyQuery("FirstOrFail"))
}

func (q policyQuery) FirstOrNew(dest any, attributes any, values ...any) error {
	panic(unsupportedPolicyQuery("FirstOrNew"))
}

func (q policyQuery) ForceDelete(value ...any) (*db.Result, error) {
	panic(unsupportedPolicyQuery("ForceDelete"))
}

func (q policyQuery) Get(dest any) error {
	panic(unsupportedPolicyQuery("Get"))
}

func (q policyQuery) Group(column string) orm.Query {
	panic(unsupportedPolicyQuery("Group"))
}

func (q policyQuery) GroupBy(column ...string) orm.Query {
	panic(unsupportedPolicyQuery("GroupBy"))
}

func (q policyQuery) Having(query any, args ...any) orm.Query {
	panic(unsupportedPolicyQuery("Having"))
}

func (q policyQuery) InRandomOrder() orm.Query {
	panic(unsupportedPolicyQuery("InRandomOrder"))
}

func (q policyQuery) InTransaction() bool {
	panic(unsupportedPolicyQuery("InTransaction"))
}

func (q policyQuery) Join(query string, args ...any) orm.Query {
	panic(unsupportedPolicyQuery("Join"))
}

func (q policyQuery) Limit(limit int) orm.Query {
	panic(unsupportedPolicyQuery("Limit"))
}

func (q policyQuery) Load(dest any, relation string, args ...any) error {
	panic(unsupportedPolicyQuery("Load"))
}

func (q policyQuery) LoadMissing(dest any, relation string, args ...any) error {
	panic(unsupportedPolicyQuery("LoadMissing"))
}

func (q policyQuery) LockForUpdate() orm.Query {
	panic(unsupportedPolicyQuery("LockForUpdate"))
}

func (q policyQuery) Model(value any) orm.Query {
	panic(unsupportedPolicyQuery("Model"))
}

func (q policyQuery) Offset(offset int) orm.Query {
	panic(unsupportedPolicyQuery("Offset"))
}

func (q policyQuery) Omit(columns ...string) orm.Query {
	panic(unsupportedPolicyQuery("Omit"))
}

func (q policyQuery) Order(value any) orm.Query {
	panic(unsupportedPolicyQuery("Order"))
}

func (q policyQuery) OrderBy(column string, direction ...string) orm.Query {
	panic(unsupportedPolicyQuery("OrderBy"))
}

func (q policyQuery) OrderByDesc(column string) orm.Query {
	panic(unsupportedPolicyQuery("OrderByDesc"))
}

func (q policyQuery) OrderByRaw(raw string) orm.Query {
	panic(unsupportedPolicyQuery("OrderByRaw"))
}

func (q policyQuery) OrWhereBetween(column string, x, y any) orm.Query {
	return q.next(q.query.OrWhereBetween(column, x, y))
}

func (q policyQuery) OrWhereIn(column string, values []any) orm.Query {
	return q.next(q.query.OrWhereIn(column, values))
}

func (q policyQuery) OrWhereJsonContains(column string, value any) orm.Query {
	return q.next(q.query.OrWhereJsonContains(column, value))
}

func (q policyQuery) OrWhereJsonContainsKey(column string) orm.Query {
	return q.next(q.query.OrWhereJsonContainsKey(column))
}

func (q policyQuery) OrWhereJsonDoesntContain(column string, value any) orm.Query {
	return q.next(q.query.OrWhereJsonDoesntContain(column, value))
}

func (q policyQuery) OrWhereJsonDoesntContainKey(column string) orm.Query {
	return q.next(q.query.OrWhereJsonDoesntContainKey(column))
}

func (q policyQuery) OrWhereJsonLength(column string, length int) orm.Query {
	return q.next(q.query.OrWhereJsonLength(column, length))
}

func (q policyQuery) OrWhereNotBetween(column string, x, y any) orm.Query {
	return q.next(q.query.OrWhereNotBetween(column, x, y))
}

func (q policyQuery) OrWhereNotIn(column string, values []any) orm.Query {
	return q.next(q.query.OrWhereNotIn(column, values))
}

func (q policyQuery) OrWhereNull(column string) orm.Query {
	return q.next(q.query.OrWhereNull(column))
}

func (q policyQuery) Paginate(page, limit int, dest any, total *int64) error {
	panic(unsupportedPolicyQuery("Paginate"))
}

func (q policyQuery) Pluck(column string, dest any) error {
	panic(unsupportedPolicyQuery("Pluck"))
}

func (q policyQuery) Raw(sql string, values ...any) orm.Query {
	panic(unsupportedPolicyQuery("Raw"))
}

func (q policyQuery) Restore(model ...any) (*db.Result, error) {
	panic(unsupportedPolicyQuery("Restore"))
}

func (q policyQuery) Rollback() error {
	panic(unsupportedPolicyQuery("Rollback"))
}

func (q policyQuery) Save(value any) error {
	panic(unsupportedPolicyQuery("Save"))
}

func (q policyQuery) SaveQuietly(value any) error {
	panic(unsupportedPolicyQuery("SaveQuietly"))
}

func (q policyQuery) Scan(dest any) error {
	panic(unsupportedPolicyQuery("Scan"))
}

func (q policyQuery) Select(columns ...string) orm.Query {
	panic(unsupportedPolicyQuery("Select"))
}

func (q policyQuery) SelectRaw(query any, args ...any) orm.Query {
	panic(unsupportedPolicyQuery("SelectRaw"))
}

func (q policyQuery) SharedLock() orm.Query {
	panic(unsupportedPolicyQuery("SharedLock"))
}

func (q policyQuery) Sum(column string, dest any) error {
	panic(unsupportedPolicyQuery("Sum"))
}

func (q policyQuery) Avg(column string, dest any) error {
	panic(unsupportedPolicyQuery("Avg"))
}

func (q policyQuery) Min(column string, dest any) error {
	panic(unsupportedPolicyQuery("Min"))
}

func (q policyQuery) Max(column string, dest any) error {
	panic(unsupportedPolicyQuery("Max"))
}

func (q policyQuery) Table(name string, args ...any) orm.Query {
	panic(unsupportedPolicyQuery("Table"))
}

func (q policyQuery) ToSql() orm.ToSql {
	panic(unsupportedPolicyQuery("ToSql"))
}

func (q policyQuery) ToRawSql() orm.ToSql {
	panic(unsupportedPolicyQuery("ToRawSql"))
}

func (q policyQuery) Update(column any, value ...any) (*db.Result, error) {
	panic(unsupportedPolicyQuery("Update"))
}

func (q policyQuery) UpdateOrCreate(dest any, attributes any, values any) error {
	panic(unsupportedPolicyQuery("UpdateOrCreate"))
}

func (q policyQuery) WhereAll(columns []string, args ...any) orm.Query {
	return q.next(q.query.WhereAll(columns, args...))
}

func (q policyQuery) WhereAny(columns []string, args ...any) orm.Query {
	return q.next(q.query.WhereAny(columns, args...))
}

func (q policyQuery) WhereBetween(column string, x, y any) orm.Query {
	return q.next(q.query.WhereBetween(column, x, y))
}

func (q policyQuery) WhereIn(column string, values []any) orm.Query {
	return q.next(q.query.WhereIn(column, values))
}

func (q policyQuery) WhereJsonContains(column string, value any) orm.Query {
	return q.next(q.query.WhereJsonContains(column, value))
}

func (q policyQuery) WhereJsonContainsKey(column string) orm.Query {
	return q.next(q.query.WhereJsonContainsKey(column))
}

func (q policyQuery) WhereJsonDoesntContain(column string, value any) orm.Query {
	return q.next(q.query.WhereJsonDoesntContain(column, value))
}

func (q policyQuery) WhereJsonDoesntContainKey(column string) orm.Query {
	return q.next(q.query.WhereJsonDoesntContainKey(column))
}

func (q policyQuery) WhereJsonLength(column string, length int) orm.Query {
	return q.next(q.query.WhereJsonLength(column, length))
}

func (q policyQuery) WhereNone(columns []string, args ...any) orm.Query {
	return q.next(q.query.WhereNone(columns, args...))
}

func (q policyQuery) WhereNotBetween(column string, x, y any) orm.Query {
	return q.next(q.query.WhereNotBetween(column, x, y))
}

func (q policyQuery) WhereNotIn(column string, values []any) orm.Query {
	return q.next(q.query.WhereNotIn(column, values))
}

func (q policyQuery) WhereNotNull(column string) orm.Query {
	return q.next(q.query.WhereNotNull(column))
}

func (q policyQuery) WhereNull(column string) orm.Query {
	return q.next(q.query.WhereNull(column))
}

func (q policyQuery) WithoutEvents() orm.Query {
	panic(unsupportedPolicyQuery("WithoutEvents"))
}

func (q policyQuery) WithoutGlobalScopes(names ...string) orm.Query {
	panic(unsupportedPolicyQuery("WithoutGlobalScopes"))
}

func (q policyQuery) WithTrashed() orm.Query {
	panic(unsupportedPolicyQuery("WithTrashed"))
}

func (q policyQuery) With(query string, args ...any) orm.Query {
	panic(unsupportedPolicyQuery("With"))
}

func (q policyQuery) WithContext(ctx context.Context) orm.Query {
	panic(unsupportedPolicyQuery("WithContext"))
}
