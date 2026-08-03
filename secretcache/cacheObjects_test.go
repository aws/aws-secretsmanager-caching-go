// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You
// may not use this file except in compliance with the License. A copy of
// the License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is
// distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific
// language governing permissions and limitations under the License.

package secretcache

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

func TestIsRefreshNeededBase(t *testing.T) {
	obj := cacheObject{refreshNeeded: true}

	if !obj.isRefreshNeeded() {
		t.Fatalf("Expected true when refreshNeeded is true")
	}

	obj.refreshNeeded = false

	if obj.isRefreshNeeded() {
		t.Fatalf("Expected false when err is nil")
	}

	obj.err = errors.New("some dummy error")

	if !obj.isRefreshNeeded() {
		t.Fatalf("Expected true when err is not nil")
	}

	obj.nextRetryTime = time.Now().Add(time.Hour * 1)

	if obj.isRefreshNeeded() {
		t.Fatalf("Expected false when nextRetryTime is in future")
	}

	obj.nextRetryTime = time.Now().Add(-(time.Hour * 1))
	if !obj.isRefreshNeeded() {
		t.Fatalf("Expected true when nextRetryTime is in past")
	}
}

func TestMaxCacheTTL(t *testing.T) {

	mockClient := dummyClient{}

	cacheItem := secretCacheItem{
		cacheObject: &cacheObject{
			secretId: "dummy-secret-name",
			client:   &mockClient,
			data: &secretsmanager.DescribeSecretOutput{
				ARN:         getStrPtr("dummy-arn"),
				Name:        getStrPtr("dummy-name"),
				Description: getStrPtr("dummy-description"),
			},
		},
	}

	config := CacheConfig{CacheItemTTL: -1}
	cacheItem.config = config

	_, err := cacheItem.executeRefresh(context.Background())

	if err == nil {
		t.Fatalf("Expected error due to negative cache ttl")
	}

	config = CacheConfig{CacheItemTTL: 0}
	cacheItem.config = config

	_, err = cacheItem.executeRefresh(context.Background())

	if err != nil {
		t.Fatalf("Unexpected error on zero cache ttl")
	}
}

func TestRefreshNow(t *testing.T) {
	mockClient := dummyClient{}

	cacheItem := secretCacheItem{
		cacheObject: &cacheObject{
			secretId: "dummy-secret-name",
			client:   &mockClient,
			data: &secretsmanager.DescribeSecretOutput{
				ARN:         getStrPtr("dummy-arn"),
				Name:        getStrPtr("dummy-name"),
				Description: getStrPtr("dummy-description"),
			},
		},
	}

	config := CacheConfig{CacheItemTTL: 0}
	cacheItem.config = config
	cacheItem.refresh(context.Background())
	refreshTime := cacheItem.nextRefreshTime

	cacheItem.refresh(context.Background())

	if refreshTime != cacheItem.nextRefreshTime {
		t.Fatalf("Expected nextRefreshTime to be same")
	}

	cacheItem.refreshNow(context.Background())

	if cacheItem.nextRefreshTime.Equal(refreshTime) {
		t.Fatalf("Expected nextRefreshTime to be different")
	}

	if cacheItem.errorCount > 0 {
		t.Fatalf("Expected errorCount to be 0")
	}

}

// Verifies TTL check uses monotonic time: no refresh before TTL, refresh after.
func TestWallClockReset_CacheItemTTL_StuckRefresh(t *testing.T) {
	clock := newFakeClock()
	mockClient := &dummyClient{}

	cacheItem := secretCacheItem{
		versions: newLRUCache(10),
		cacheObject: &cacheObject{
			secretId:      "dummy-secret-name",
			client:        mockClient,
			refreshNeeded: false,
			now:           clock.Now,
			nowWall:       clock.NowWall,
			data: &secretsmanager.DescribeSecretOutput{
				ARN:  getStrPtr("dummy-arn"),
				Name: getStrPtr("dummy-name"),
			},
		},
		nextRefreshTime: clock.Now().Add(time.Hour),
	}

	if cacheItem.isRefreshNeeded() {
		t.Fatalf("Expected no refresh needed when TTL has not expired")
	}

	// Advance only the monotonic clock, so the wall clock cannot be what
	// triggers the refresh below.
	clock.AdvanceMonotonic(30 * time.Minute)
	if cacheItem.isRefreshNeeded() {
		t.Fatalf("Expected no refresh needed — only 30 minutes elapsed, TTL is 1 hour")
	}

	clock.AdvanceMonotonic(time.Hour)
	if !cacheItem.isRefreshNeeded() {
		t.Fatalf("Expected refresh needed — 1h30m elapsed, exceeds 1 hour TTL")
	}
}

// Verifies exponential backoff respects the injectable clock.
func TestWallClockReset_ErrorBackoff_UsesCorrectClock(t *testing.T) {
	clock := newFakeClock()
	callCount := 0
	failingClient := &failingDummyClient{describeCallCount: &callCount}

	cacheItem := secretCacheItem{
		versions: newLRUCache(10),
		cacheObject: &cacheObject{
			secretId:      "dummy-secret-name",
			client:        failingClient,
			refreshNeeded: true,
			now:           clock.Now,
			nowWall:       clock.NowWall,
		},
		nextRefreshTime: clock.Now(),
	}

	cacheItem.refresh(context.Background())
	if callCount != 1 {
		t.Fatalf("Expected 1 call, got %d", callCount)
	}
	if cacheItem.err == nil {
		t.Fatalf("Expected error to be set")
	}

	if cacheItem.cacheObject.isRefreshNeeded() {
		t.Fatalf("Expected no refresh during backoff period")
	}

	clock.AdvanceMonotonic(time.Millisecond)
	if cacheItem.cacheObject.isRefreshNeeded() {
		t.Fatalf("Expected no refresh during backoff — only 1ms elapsed, backoff is 2ms")
	}

	clock.AdvanceMonotonic(time.Millisecond)
	if !cacheItem.cacheObject.isRefreshNeeded() {
		t.Fatalf("Expected refresh needed, exceeds 2ms backoff")
	}

	cacheItem.refresh(context.Background())
	if callCount != 2 {
		t.Fatalf("Expected 2 calls, got %d", callCount)
	}
}

// Regression: refreshNow must not block when nextRefreshTime is in the past.
func TestWallClockReset_RefreshNow_DoesNotBlock(t *testing.T) {
	clock := newFakeClock()
	mockClient := &dummyClient{}

	// Simulate Wall clock being set 24 hours back in time
	cacheItem := secretCacheItem{
		versions: newLRUCache(10),
		cacheObject: &cacheObject{
			secretId:      "dummy-secret-name",
			client:        mockClient,
			refreshNeeded: false,
			err:           errors.New("previous API failure"),
			errorCount:    3,
			now:           clock.Now,
			nowWall:       clock.NowWall,
		},
		nextRefreshTime: time.Now().Add(24 * time.Hour),
	}
	clock.AdvanceMonotonic(24 * time.Hour)

	// Verify that it will refresh within 6 seconds
	// since the monotonic time should be correct
	done := make(chan struct{})
	go func() {
		cacheItem.refreshNow(context.Background())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(6 * time.Second):
		t.Fatalf("refreshNow blocked longer than 6 seconds")
	}
}

// Wall clock fallback catches staleness when monotonic clock freezes (macOS sleep).
func TestDualCheck_WallClockFallback_MonotonicFrozen(t *testing.T) {
	clock := newFakeClock()
	mockClient := &dummyClient{}

	cacheItem := secretCacheItem{
		versions: newLRUCache(10),
		cacheObject: &cacheObject{
			secretId:      "dummy-secret-name",
			client:        mockClient,
			refreshNeeded: false,
			now:           clock.Now,
			nowWall:       clock.NowWall,
			data: &secretsmanager.DescribeSecretOutput{
				ARN:  getStrPtr("dummy-arn"),
				Name: getStrPtr("dummy-name"),
			},
		},
		// TTL expires an hour from now by both clocks.
		nextRefreshTime: clock.Now().Add(time.Hour),
	}

	if cacheItem.isRefreshNeeded() {
		t.Fatalf("Expected no refresh needed — TTL has not expired on either clock")
	}

	// Simulate the host suspending for 24 hours: the monotonic clock freezes
	// where it is while the wall clock keeps advancing. The monotonic branch
	// still sees the TTL as an hour away, so only the wall clock fallback can
	// catch that the secret is now stale.
	clock.AdvanceWall(24 * time.Hour)

	if cacheItem.nextRefreshTime.Compare(clock.Now()) <= 0 {
		t.Fatalf("Test precondition broken: monotonic branch should not see the TTL as expired")
	}

	if !cacheItem.isRefreshNeeded() {
		t.Fatalf("Expected refresh needed — wall clock advanced 24h past the TTL")
	}
}

// Wall clock fallback catches an elapsed retry backoff when the monotonic clock
// freezes (macOS sleep) after error.
func TestDualCheck_ErrorRetryTime_WallClockFallback_MonotonicFrozen(t *testing.T) {
	clock := newFakeClock()
	callCount := 0
	failingClient := &failingDummyClient{describeCallCount: &callCount}

	cacheItem := secretCacheItem{
		versions: newLRUCache(10),
		cacheObject: &cacheObject{
			secretId:      "dummy-secret-name",
			client:        failingClient,
			refreshNeeded: true,
			now:           clock.Now,
			nowWall:       clock.NowWall,
		},
		nextRefreshTime: clock.Now(),
	}

	// Fail a refresh so err is set and nextRetryTime is armed.
	cacheItem.refresh(context.Background())

	if cacheItem.err == nil {
		t.Fatalf("Expected error to be set")
	}

	if cacheItem.nextRetryTime.IsZero() {
		t.Fatalf("Expected nextRetryTime to be armed")
	}

	if cacheItem.cacheObject.isRefreshNeeded() {
		t.Fatalf("Expected no refresh — backoff has not elapsed on either clock")
	}

	// Simulate the monotonic clock freezing by advancing the wall clock
	clock.AdvanceWall(24 * time.Hour)

	if cacheItem.nextRetryTime.Compare(clock.Now()) <= 0 {
		t.Fatalf("Test precondition broken: monotonic branch should not see the backoff as elapsed")
	}

	if !cacheItem.cacheObject.isRefreshNeeded() {
		t.Fatalf("Expected refresh needed — wall clock advanced 24h past the backoff")
	}
}

// fakeClock models the monotonic and wall clocks as separate offsets so tests
// can advance them independently. Wire Now into cacheObject.now and NowWall
// into cacheObject.nowWall.
type fakeClock struct {
	base            time.Time
	monotonicOffset time.Duration
	wallOffset      time.Duration
}

func newFakeClock() *fakeClock {
	return &fakeClock{base: time.Now()}
}

// Now is the monotonic reading. Only monotonicOffset moves it.
func (fc *fakeClock) Now() time.Time {
	return fc.base.Add(fc.monotonicOffset)
}

// NowWall is the wall clock reading, with the monotonic reading stripped so
// comparisons against it use the wall clock. Only wallOffset moves it.
func (fc *fakeClock) NowWall() time.Time {
	return fc.base.Round(0).Add(fc.wallOffset)
}

// AdvanceMonotonic moves the monotonic clock forward, leaving the wall clock
// where it is.
func (fc *fakeClock) AdvanceMonotonic(d time.Duration) {
	fc.monotonicOffset += d
}

// AdvanceWall moves the wall clock forward, leaving the monotonic clock where it
// is. This is what happens when the host suspends (e.g. macOS sleep) and the
// monotonic clock freezes while the wall clock keeps going.
func (fc *fakeClock) AdvanceWall(d time.Duration) {
	fc.wallOffset += d
}

type failingDummyClient struct {
	SecretsManagerAPIClient
	describeCallCount *int
}

func (f *failingDummyClient) DescribeSecret(context context.Context, input *secretsmanager.DescribeSecretInput, opts ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error) {
	*f.describeCallCount++
	return nil, errors.New("service unavailable")
}

type dummyClient struct {
	SecretsManagerAPIClient
}

func (d *dummyClient) DescribeSecret(context context.Context, input *secretsmanager.DescribeSecretInput, opts ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error) {
	return &secretsmanager.DescribeSecretOutput{}, nil
}

type countingDummyClient struct {
	SecretsManagerAPIClient
	describeCallCount *int
}

func (c *countingDummyClient) DescribeSecret(context context.Context, input *secretsmanager.DescribeSecretInput, opts ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error) {
	*c.describeCallCount++
	return &secretsmanager.DescribeSecretOutput{}, nil
}

// Helper function to get a string pointer for input string.
func getStrPtr(str string) *string {
	return &str
}
