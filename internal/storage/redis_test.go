package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"gh.tarampamp.am/webhook-tester/v2/internal/storage"
)

func TestRedis_Session_CreateReadDelete(t *testing.T) {
	t.Parallel()

	var (
		mini = miniredis.RunT(t)
		ft   = newFakeTime(t)
	)

	testSessionCreateReadDelete(t,
		func(sTTL time.Duration, maxReq uint32) storage.Storage {
			return storage.NewRedis(
				redis.NewClient(&redis.Options{Addr: mini.Addr()}),
				sTTL,
				maxReq,
				storage.WithRedisTimeNow(ft.Get),
			)
		},
		func(t time.Duration) { mini.FastForward(t); ft.Add(t) },
		ft.Get,
	)
}

func TestRedis_Request_CreateReadDelete(t *testing.T) {
	t.Parallel()

	var (
		mini = miniredis.RunT(t)
		ft   = newFakeTime(t)
	)

	testRequestCreateReadDelete(t,
		func(sTTL time.Duration, maxReq uint32) storage.Storage {
			return storage.NewRedis(
				redis.NewClient(&redis.Options{Addr: mini.Addr()}),
				sTTL,
				maxReq,
				storage.WithRedisTimeNow(ft.Get),
			)
		},
		func(t time.Duration) { mini.FastForward(t); ft.Add(t) },
	)
}

// TestRedis_RequestKeysShareSessionLifetime is a regression test for two TTL defects in the Redis storage:
//
//   - the requests index (a zset) was created by ZAdd without any expiry, so it outlived its session forever
//     (a permanent key leak for every single-request session);
//   - request data keys were stored with the fixed base session TTL instead of the session's current (extended)
//     lifetime, so the newest request - and any files it uploaded - could expire before the session did.
//
// Both are fixed by aligning the request data key and the index with the session's current PTTL in NewRequest.
func TestRedis_RequestKeysShareSessionLifetime(t *testing.T) {
	t.Parallel()

	const sTTL = time.Hour

	var (
		mini = miniredis.RunT(t)
		ctx  = context.Background()
		db   = storage.NewRedis(redis.NewClient(&redis.Options{Addr: mini.Addr()}), sTTL, 128)
	)

	sID, err := db.NewSession(ctx, storage.Session{})
	require.NoError(t, err)

	var (
		sessionKey  = "webhook-tester-v2:session:" + sID
		requestsKey = sessionKey + ":requests"
	)

	// first request of the session: the index must receive a TTL (Flaw 1 - previously it had none)
	rID1, err := db.NewRequest(ctx, sID, storage.Request{Method: "GET"})
	require.NoError(t, err)

	require.Positive(t, mini.TTL(requestsKey), "the requests index must inherit an expiry, not live forever")
	require.Positive(t, mini.TTL(sessionKey+":requests:"+rID1))

	// extend the session lifetime (as the webhook middleware does on every incoming request)
	require.NoError(t, db.AddSessionTTL(ctx, sID, 2*sTTL)) // session TTL is now ~3x sTTL

	// a request stored into the now-extended session must inherit the extended lifetime, not the base sTTL
	// (Flaw 2 - otherwise the newest request and its uploaded files would expire while the session lives on)
	rID2, err := db.NewRequest(ctx, sID, storage.Request{Method: "POST"})
	require.NoError(t, err)

	require.Greater(t, mini.TTL(sessionKey+":requests:"+rID2), sTTL,
		"a request in an extended session must live as long as the session, not just the base TTL")
	require.Greater(t, mini.TTL(requestsKey), sTTL, "the index must track the extended session lifetime")

	// the index must never outlive the session
	require.LessOrEqual(t, mini.TTL(requestsKey), mini.TTL(sessionKey))
}

//	func TestRedis_RaceProvocation(t *testing.T) {
//		t.Parallel()
//
//		var mini = miniredis.RunT(t)
//
//		testRaceProvocation(t, func(sTTL time.Duration, maxReq uint32) storage.Storage {
//			return storage.NewRedis(
//				redis.NewClient(&redis.Options{Addr: mini.Addr()}),
//				encDec,
//				sTTL,
//				maxReq,
//			)
//		})
//	}
