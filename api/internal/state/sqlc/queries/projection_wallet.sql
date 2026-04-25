-- name: UpsertProjectionWalletConfig :exec
insert into mistypass_wallet_configs (id, tenant_id, provider, issuer_id, service_account_email, key_ref, status, created_at, updated_at, raw, synced_at)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, now())
on conflict (id) do update
set tenant_id = excluded.tenant_id,
    provider = excluded.provider,
    issuer_id = excluded.issuer_id,
    service_account_email = excluded.service_account_email,
    key_ref = excluded.key_ref,
    status = excluded.status,
    created_at = excluded.created_at,
    updated_at = excluded.updated_at,
    raw = excluded.raw,
    synced_at = now();

-- name: UpsertProjectionWalletTemplate :exec
insert into mistypass_wallet_templates (id, tenant_id, provider, pass_type, class_id, name, status, style_config, created_at, updated_at, raw, synced_at)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, now())
on conflict (id) do update
set tenant_id = excluded.tenant_id,
    provider = excluded.provider,
    pass_type = excluded.pass_type,
    class_id = excluded.class_id,
    name = excluded.name,
    status = excluded.status,
    style_config = excluded.style_config,
    created_at = excluded.created_at,
    updated_at = excluded.updated_at,
    raw = excluded.raw,
    synced_at = now();

-- name: UpsertProjectionWalletPass :exec
insert into mistypass_wallet_passes (id, tenant_id, provider, template_id, target_type, target_id, object_id, status, save_link, expires_at, issued_at, activated_at, revoked_at, created_by, updated_by, created_at, updated_at, raw, synced_at)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, now())
on conflict (id) do update
set tenant_id = excluded.tenant_id,
    provider = excluded.provider,
    template_id = excluded.template_id,
    target_type = excluded.target_type,
    target_id = excluded.target_id,
    object_id = excluded.object_id,
    status = excluded.status,
    save_link = excluded.save_link,
    expires_at = excluded.expires_at,
    issued_at = excluded.issued_at,
    activated_at = excluded.activated_at,
    revoked_at = excluded.revoked_at,
    created_by = excluded.created_by,
    updated_by = excluded.updated_by,
    created_at = excluded.created_at,
    updated_at = excluded.updated_at,
    raw = excluded.raw,
    synced_at = now();

-- name: UpsertProjectionWalletJob :exec
insert into mistypass_wallet_jobs (id, tenant_id, provider, batch_id, template_id, target_type, target_id, expires_at, pass_id, status, retry_count, error_code, error_message, created_at, updated_at, raw, synced_at)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, now())
on conflict (id) do update
set tenant_id = excluded.tenant_id,
    provider = excluded.provider,
    batch_id = excluded.batch_id,
    template_id = excluded.template_id,
    target_type = excluded.target_type,
    target_id = excluded.target_id,
    expires_at = excluded.expires_at,
    pass_id = excluded.pass_id,
    status = excluded.status,
    retry_count = excluded.retry_count,
    error_code = excluded.error_code,
    error_message = excluded.error_message,
    created_at = excluded.created_at,
    updated_at = excluded.updated_at,
    raw = excluded.raw,
    synced_at = now();

-- name: UpsertProjectionWalletAuditLog :exec
insert into mistypass_wallet_audit_logs (id, tenant_id, action, actor, target_id, result, at, raw, synced_at)
values ($1, $2, $3, $4, $5, $6, $7, $8, now())
on conflict (id) do update
set tenant_id = excluded.tenant_id,
    action = excluded.action,
    actor = excluded.actor,
    target_id = excluded.target_id,
    result = excluded.result,
    at = excluded.at,
    raw = excluded.raw,
    synced_at = now();
