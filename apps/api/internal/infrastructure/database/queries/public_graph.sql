-- name: GetPublicFriendGraph :one
SELECT directory.public_friend_graph(sqlc.narg(center_id)::uuid)::jsonb AS graph;
