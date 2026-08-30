package repo

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ruslan/video-offers/internal/domain"
	"github.com/ruslan/video-offers/internal/pkg/pagination"
)

type UserRepo struct {
	pool *pgxpool.Pool
}

func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

func (r *UserRepo) Create(ctx context.Context, u domain.User) error {
	const q = `
		INSERT INTO users (id, email, username, password_hash, role, display_name, avatar_url, created_at, updated_at)
		VALUES ($1, lower($2), lower($3), $4, $5, $6, $7, $8, $9)`

	_, err := r.pool.Exec(ctx, q,
		u.ID, u.Email, u.Username, u.PasswordHash, u.Role,
		u.DisplayName, u.AvatarURL, u.CreatedAt, u.UpdatedAt,
	)
	if err != nil {
		return mapUserCreateError(err)
	}
	return nil
}

func mapUserCreateError(err error) error {
	if IsUniqueViolation(err, "users_email_key") {
		return domain.ErrConflict.WithCode("email_taken", "email уже занят").
			WithDetails(map[string]any{"field": "email"})
	}
	if IsUniqueViolation(err, "users_username_key") {
		return domain.ErrConflict.WithCode("username_taken", "username уже занят").
			WithDetails(map[string]any{"field": "username"})
	}
	return MapError(err)
}

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	const q = `
		SELECT id, email, username, password_hash, role, display_name, avatar_url, created_at, updated_at
		FROM users WHERE id = $1`

	var u domain.User
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&u.ID, &u.Email, &u.Username, &u.PasswordHash, &u.Role,
		&u.DisplayName, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return domain.User{}, MapError(err)
	}
	return u, nil
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (domain.User, error) {
	const q = `
		SELECT id, email, username, password_hash, role, display_name, avatar_url, created_at, updated_at
		FROM users WHERE lower(email) = lower($1)`

	var u domain.User
	err := r.pool.QueryRow(ctx, q, strings.TrimSpace(email)).Scan(
		&u.ID, &u.Email, &u.Username, &u.PasswordHash, &u.Role,
		&u.DisplayName, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return domain.User{}, MapError(err)
	}
	return u, nil
}

func (r *UserRepo) GetByUsername(ctx context.Context, username string) (domain.User, error) {
	const q = `
		SELECT id, email, username, password_hash, role, display_name, avatar_url, created_at, updated_at
		FROM users WHERE lower(username) = lower($1)`

	var u domain.User
	err := r.pool.QueryRow(ctx, q, strings.TrimSpace(username)).Scan(
		&u.ID, &u.Email, &u.Username, &u.PasswordHash, &u.Role,
		&u.DisplayName, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return domain.User{}, MapError(err)
	}
	return u, nil
}

func (r *UserRepo) Update(ctx context.Context, u domain.User) error {
	const q = `
		UPDATE users
		SET display_name = $2, avatar_url = $3, role = $4, updated_at = $5
		WHERE id = $1`

	tag, err := r.pool.Exec(ctx, q, u.ID, u.DisplayName, u.AvatarURL, u.Role, u.UpdatedAt)
	if err != nil {
		return MapError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// StreamerListItem — стример с настройками для публичного списка.
type StreamerListItem struct {
	User            domain.User
	AcceptingOffers bool
}

type ListStreamersParams struct {
	UsernamePrefix string
	Cursor         *pagination.Key
	Limit          int
}

func (r *UserRepo) ListStreamers(ctx context.Context, p ListStreamersParams) ([]StreamerListItem, error) {
	limit := p.Limit + 1

	args := []any{domain.RoleStreamer, p.UsernamePrefix}
	q := `
		SELECT u.id, u.email, u.username, u.password_hash, u.role, u.display_name, u.avatar_url,
		       u.created_at, u.updated_at, ss.accepting_offers
		FROM users u
		INNER JOIN streamer_settings ss ON ss.user_id = u.id
		WHERE u.role = $1
		  AND ($2 = '' OR u.username LIKE lower($2) || '%')`

	if p.Cursor != nil {
		args = append(args, p.Cursor.CreatedAt, p.Cursor.ID)
		q += fmt.Sprintf(`
		  AND (u.created_at, u.id) < ($%d, $%d)`, len(args)-1, len(args))
	}

	args = append(args, limit)
	q += fmt.Sprintf(`
		ORDER BY u.created_at DESC, u.id DESC
		LIMIT $%d`, len(args))

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, MapError(err)
	}
	defer rows.Close()

	var items []StreamerListItem
	for rows.Next() {
		var item StreamerListItem
		if err := rows.Scan(
			&item.User.ID, &item.User.Email, &item.User.Username, &item.User.PasswordHash,
			&item.User.Role, &item.User.DisplayName, &item.User.AvatarURL,
			&item.User.CreatedAt, &item.User.UpdatedAt, &item.AcceptingOffers,
		); err != nil {
			return nil, MapError(err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, MapError(err)
	}
	return items, nil
}
