package message

import (
	"context"
	"log/slog"
	"maps"
	"net"
	"sync/atomic"
	"time"
)

// DeviceNamer provides a method to get a device name from a device index. The
// DeviceName method returns the name and true if the device index exists. Note
// that the name may be empty if the device does not have a configured name.
type DeviceNamer interface {
	DeviceName(index int) (name string, ok bool)
}

// ListingDeviceNamer lists all devices on the system and maintains a cache for
// lookups. It refreshes the device list periodically or on the first lookup
// for an unknown index.
type ListingDeviceNamer struct {
	ctx     context.Context
	cache   atomic.Pointer[deviceCache]
	refresh chan refreshRequest
}

// NewListingDeviceNamer creates a DeviceNamer that uses a list of devices
// updated approximately every interval.
func NewListingDeviceNamer(ctx context.Context, interval time.Duration) *ListingDeviceNamer {
	dn := ListingDeviceNamer{
		ctx:     ctx,
		refresh: make(chan refreshRequest),
	}

	dn.cache.Store(refreshDeviceCache(ctx, nil, refreshRequest{}))

	go func() {
		for {
			select {
			case req := <-dn.refresh:
				existing := dn.cache.Load()
				dn.cache.Store(refreshDeviceCache(ctx, existing, req))
				if req.done != nil {
					close(req.done)
				}

			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		ticker := time.NewTicker(interval)
		for {
			select {
			case <-ticker.C:
				// Only refresh if the last non-ticker refresh was within 25%
				// of the interval. This limits the maximum age of the cache to
				// 125% of the interval.
				dn.refresh <- refreshRequest{maxAge: interval / 4}

			case <-ctx.Done():
				return
			}
		}
	}()

	return &dn
}

func (dn *ListingDeviceNamer) DeviceName(index int) (string, bool) {
	cache := dn.cache.Load()
	if name, ok := cache.names[index]; ok {
		return name, true
	}
	if _, missing := cache.missing[index]; missing {
		return "", false
	}

	done := make(chan struct{})
	select {
	case dn.refresh <- refreshRequest{index: index, done: done}:
	case <-dn.ctx.Done():
		// Don't block on a refresh goroutine that already exited
		return "", false
	}

	// Wait for the refresh to complete
	<-done
	cache = dn.cache.Load()

	name, ok := cache.names[index]
	return name, ok
}

type deviceCache struct {
	updatedAt time.Time
	names     map[int]string
	missing   map[int]bool
}

type refreshRequest struct {
	// The optional index of the device that triggered the refresh
	index int

	// The maximum allowed age of the existing cache. If set, skip the refresh
	// if the cache is newer than this.
	maxAge time.Duration

	// An optional channel closed when the refresh is complete
	done chan struct{}
}

func refreshDeviceCache(ctx context.Context, existing *deviceCache, req refreshRequest) *deviceCache {
	cache := deviceCache{
		updatedAt: time.Now(),
	}

	if existing == nil {
		existing = &cache
	}

	if req.maxAge > 0 && time.Since(existing.updatedAt) < req.maxAge {
		return existing
	}

	interfaces, err := net.Interfaces()
	if err != nil {
		slog.ErrorContext(ctx, "error listing network interfaces", "error", err)

		// If the refresh was for a specific device index, cache that as a miss
		// even though it isn't confirmed. This prevents the next lookup for
		// this device from immediately triggering another refresh. Check if
		// the device actually exists on the next scheduled refresh.
		if req.index > 0 {
			cache.names = maps.Clone(existing.names)
			cache.missing = maps.Clone(existing.missing)
			cache.missing[req.index] = true
			return &cache
		}

		return existing
	}

	cache.names = make(map[int]string)
	for _, iface := range interfaces {
		cache.names[iface.Index] = iface.Name
	}

	// Copy any still-missing devices from the existing cache
	cache.missing = make(map[int]bool)
	for index := range existing.missing {
		if _, exists := cache.names[index]; !exists {
			cache.missing[index] = true
		}
	}
	// If a device was requested and is missing, cache the miss
	if req.index > 0 {
		if _, exists := cache.names[req.index]; !exists {
			cache.missing[req.index] = true
		}
	}

	return &cache
}
