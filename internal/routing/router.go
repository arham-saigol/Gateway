package routing

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"arham-gateway/internal/config"
	"arham-gateway/internal/crypto"
	"arham-gateway/internal/database"
	"arham-gateway/internal/providers"
)

var (
	ErrNoRoutesAvailable  = errors.New("no enabled routes configured for model")
	ErrNoHealthyKeysFound = errors.New("no healthy provider keys available for model")
)

type Target struct {
	PublicModelID      string
	ProviderID         string
	ProviderKeyID      string
	MappingID          string
	UpstreamModelID    string
	DecryptedSecret    string
	InputRateSnapshot  int64
	CachedRateSnapshot int64
	OutputRateSnapshot int64
	SupportsStreaming  bool
	SupportsTools      bool
}

type Router struct {
	db          *database.DB
	masterKey   []byte
	cfg         *config.Config
	adapters    map[string]providers.ProviderAdapter
	mu          sync.Mutex
	cooldowns   map[string]time.Time
	keyCounters map[string]uint64
}

func NewRouter(db *database.DB, masterKey []byte, cfg *config.Config) *Router {
	r := &Router{
		db:          db,
		masterKey:   masterKey,
		cfg:         cfg,
		adapters:    make(map[string]providers.ProviderAdapter),
		cooldowns:   make(map[string]time.Time),
		keyCounters: make(map[string]uint64),
	}

	dialTimeout := 10 * time.Second
	if cfg != nil && cfg.Timeouts.UpstreamDialTimeout.Duration() > 0 {
		dialTimeout = cfg.Timeouts.UpstreamDialTimeout.Duration()
	}

	// Register built-in adapters
	r.RegisterAdapter(providers.NewFireworksAdapter(dialTimeout))
	r.RegisterAdapter(providers.NewSiliconFlowAdapter(dialTimeout))
	r.RegisterAdapter(providers.NewNovitaAdapter(dialTimeout))
	r.RegisterAdapter(providers.NewBasetenAdapter(dialTimeout))

	return r
}

func (r *Router) RegisterAdapter(adapter providers.ProviderAdapter) {
	r.adapters[adapter.ID()] = adapter
}

func (r *Router) GetAdapter(providerID string) (providers.ProviderAdapter, bool) {
	a, ok := r.adapters[providerID]
	return a, ok
}

func (r *Router) MarkKeyCooldown(keyID string, duration time.Duration) {
	if duration <= 0 {
		duration = r.cfg.Routing.KeyCooldownDuration.Duration()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cooldowns[keyID] = time.Now().UTC().Add(duration)
}

func (r *Router) IsKeyInCooldown(keyID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	until, ok := r.cooldowns[keyID]
	if !ok {
		return false
	}
	if time.Now().UTC().Before(until) {
		return true
	}
	delete(r.cooldowns, keyID)
	return false
}

func (r *Router) MarkKeyAuthInvalid(keyID string, reason string) error {
	r.mu.Lock()
	delete(r.cooldowns, keyID)
	r.mu.Unlock()

	safeErr := "Authentication failed: key is invalid or revoked upstream"
	if reason != "" {
		safeErr = reason
	}
	return r.db.UpdateProviderKeyStatus(keyID, "invalid", &safeErr)
}

func (r *Router) SelectNextTarget(publicModelID string, attemptedKeyIDs map[string]bool) (*Target, error) {
	routes, err := r.db.GetRoutesForModel(publicModelID)
	if err != nil {
		return nil, fmt.Errorf("retrieving routes for %s: %w", publicModelID, err)
	}

	if len(routes) == 0 {
		return nil, ErrNoRoutesAvailable
	}

	// Iterate through routes in priority order
	for _, route := range routes {
		if !route.Enabled {
			continue
		}

		mapping, err := r.db.GetProviderModelMapping(route.MappingID)
		if err != nil || !mapping.Enabled {
			continue
		}

		// Retrieve keys for this provider
		keys, err := r.db.ListProviderKeys(route.ProviderID)
		if err != nil || len(keys) == 0 {
			continue
		}

		// Filter active, healthy, unattempted keys
		var availableKeys []database.ProviderKey
		for _, k := range keys {
			if k.Status != "active" {
				continue
			}
			if attemptedKeyIDs != nil && attemptedKeyIDs[k.ID] {
				continue
			}
			if r.IsKeyInCooldown(k.ID) {
				continue
			}
			availableKeys = append(availableKeys, k)
		}

		if len(availableKeys) == 0 {
			continue
		}

		// Select key via round-robin index
		r.mu.Lock()
		idx := int(r.keyCounters[route.ProviderID] % uint64(len(availableKeys)))
		r.keyCounters[route.ProviderID]++
		r.mu.Unlock()

		selectedKey := availableKeys[idx]

		// Decrypt secret
		decrypted, err := crypto.Decrypt(r.masterKey, selectedKey.EncryptedSecret)
		if err != nil {
			return nil, fmt.Errorf("decrypting secret for key %s: %w", selectedKey.ID, err)
		}

		return &Target{
			PublicModelID:      publicModelID,
			ProviderID:         route.ProviderID,
			ProviderKeyID:      selectedKey.ID,
			MappingID:          mapping.ID,
			UpstreamModelID:    mapping.UpstreamModelID,
			DecryptedSecret:    decrypted,
			InputRateSnapshot:  mapping.InputRatePerMTokens,
			CachedRateSnapshot: mapping.CachedRatePerMTokens,
			OutputRateSnapshot: mapping.OutputRatePerMTokens,
			SupportsStreaming:  mapping.SupportsStreaming,
			SupportsTools:      mapping.SupportsTools,
		}, nil
	}

	return nil, ErrNoHealthyKeysFound
}
