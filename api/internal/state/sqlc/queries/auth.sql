-- name: UpsertAuthUser :exec
insert into mistypass_auth_users (id, email, role, tenant_id, building_ids, password_hash, created_at, updated_at)
values ($1, $2, $3, $4, $5, $6, now(), now())
on conflict (id) do update
set email = excluded.email,
    role = excluded.role,
    tenant_id = excluded.tenant_id,
    building_ids = excluded.building_ids,
    password_hash = excluded.password_hash,
    updated_at = now();

-- name: GetAuthUserByEmail :one
select id, email, role, tenant_id, building_ids, password_hash
from mistypass_auth_users
where lower(email) = $1;

-- name: GetAuthUserByID :one
select id, email, role, tenant_id, building_ids, password_hash
from mistypass_auth_users
where id = $1;

-- name: UpsertAuthRefreshSession :exec
insert into mistypass_auth_refresh_sessions (session_id, user_id, expires_at, created_at, updated_at)
values ($1, $2, $3, now(), now())
on conflict (session_id) do update
set user_id = excluded.user_id,
    expires_at = excluded.expires_at,
    updated_at = now();

-- name: GetAuthRefreshSession :one
select user_id, expires_at
from mistypass_auth_refresh_sessions
where session_id = $1;

-- name: DeleteAuthRefreshSession :exec
delete from mistypass_auth_refresh_sessions where session_id = $1;

-- name: DeleteAuthRefreshSessionsByUserID :execrows
delete from mistypass_auth_refresh_sessions where user_id = $1;

-- name: UpsertAuthRevokedAccessToken :exec
insert into mistypass_auth_revoked_access_tokens (token_id, expires_at, created_at, updated_at)
values ($1, $2, now(), now())
on conflict (token_id) do update
set expires_at = excluded.expires_at,
    updated_at = now();

-- name: GetAuthRevokedAccessTokenExpiresAt :one
select expires_at
from mistypass_auth_revoked_access_tokens
where token_id = $1;

-- name: DeleteExpiredAuthRevokedAccessToken :exec
delete from mistypass_auth_revoked_access_tokens
where token_id = $1 and expires_at <= $2;

-- name: UpsertGatewayDeviceToken :exec
insert into mistypass_gateway_device_tokens (gateway_id, token_hash, created_at, updated_at)
values ($1, $2, now(), now())
on conflict (gateway_id) do update
set token_hash = excluded.token_hash,
    updated_at = now();

-- name: GetGatewayDeviceTokenHash :one
select token_hash
from mistypass_gateway_device_tokens
where gateway_id = $1;
