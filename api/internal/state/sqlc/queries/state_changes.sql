-- name: GetStatePayload :one
select payload
from mistypass
where state_key = $1;

-- name: GetStatePayloadForUpdate :one
select payload
from mistypass
where state_key = $1
for update;

-- name: UpsertStatePayload :exec
insert into mistypass (state_key, payload, updated_at)
values ($1, $2, now())
on conflict (state_key) do update
set payload = excluded.payload,
    updated_at = now();

-- name: InsertStateChange :one
insert into mistypass_change_log (state_key, change_type, payload_hash, payload, created_at)
values ($1, $2, $3, $4, now())
returning id;

-- name: ListStateChangesAll :many
select id, state_key, change_type, payload_hash, payload, created_at
from mistypass_change_log
order by id desc
limit $1;

-- name: ListStateChangesByKey :many
select id, state_key, change_type, payload_hash, payload, created_at
from mistypass_change_log
where state_key = $1
order by id desc
limit $2;

-- name: ListReplayPayloadsByKeyFromID :many
select id, payload
from mistypass_change_log
where state_key = $1 and id > $2
order by id asc
limit $3;

-- name: ListReplayCheckpointsAll :many
select state_key, last_change_id, updated_at
from mistypass_change_replay_checkpoints
order by updated_at desc, state_key asc
limit $1;

-- name: ListReplayCheckpointsByKey :many
select state_key, last_change_id, updated_at
from mistypass_change_replay_checkpoints
where state_key = $1
order by updated_at desc
limit $2;

-- name: GetReplayCheckpointByKey :one
select state_key, last_change_id, updated_at
from mistypass_change_replay_checkpoints
where state_key = $1;

-- name: UpsertReplayCheckpoint :one
insert into mistypass_change_replay_checkpoints (state_key, last_change_id, updated_at)
values ($1, $2, now())
on conflict (state_key) do update
set last_change_id = greatest(mistypass_change_replay_checkpoints.last_change_id, excluded.last_change_id),
    updated_at = now()
returning state_key, last_change_id, updated_at;
