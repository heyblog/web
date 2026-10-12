-- name: GetPublicFriendGraph :one
SELECT directory.public_friend_graph(sqlc.narg(center_id)::uuid)::jsonb AS graph;

-- name: GetPublicGraphRevision :one
SELECT revision FROM runtime_cache.public_graph_revision WHERE singleton;
