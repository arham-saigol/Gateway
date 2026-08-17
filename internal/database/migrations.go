package database

import (
	"cmp"
	"embed"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

func (db *DB) Migrate() error {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("reading migrations directory: %w", err)
	}

	// Create schema_migrations table if not exists
	_, err = db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TIMESTAMP NOT NULL
		);
	`)
	if err != nil {
		return fmt.Errorf("creating schema_migrations table: %w", err)
	}

	// Sort migration files numerically
	type migrationFile struct {
		version  int
		fileName string
	}
	var files []migrationFile

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		parts := strings.SplitN(entry.Name(), "_", 2)
		if len(parts) < 2 {
			continue
		}
		version, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		files = append(files, migrationFile{version: version, fileName: entry.Name()})
	}

	slices.SortFunc(files, func(a, b migrationFile) int {
		return cmp.Compare(a.version, b.version)
	})

	for _, file := range files {
		var applied int
		err := db.conn.QueryRow("SELECT COUNT(1) FROM schema_migrations WHERE version = ?", file.version).Scan(&applied)
		if err != nil {
			return fmt.Errorf("checking migration status for version %d: %w", file.version, err)
		}

		if applied > 0 {
			continue
		}

		content, err := migrationFS.ReadFile("migrations/" + file.fileName)
		if err != nil {
			return fmt.Errorf("reading migration file %s: %w", file.fileName, err)
		}

		tx, err := db.conn.Begin()
		if err != nil {
			return fmt.Errorf("beginning transaction for migration %d: %w", file.version, err)
		}

		if _, err := tx.Exec(string(content)); err != nil {
			tx.Rollback()
			return fmt.Errorf("executing migration %s: %w", file.fileName, err)
		}

		if _, err := tx.Exec("INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)", file.version, time.Now().UTC()); err != nil {
			tx.Rollback()
			return fmt.Errorf("recording migration %d: %w", file.version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("committing migration %d: %w", file.version, err)
		}
	}

	return nil
}

func (db *DB) SeedDefaults() error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().UTC()

	// Seed Providers
	providers := []struct {
		id   string
		name string
	}{
		{"fireworks", "Fireworks AI"},
		{"siliconflow", "SiliconFlow"},
		{"novita", "Novita AI"},
		{"baseten", "Baseten"},
	}

	for _, p := range providers {
		_, err := tx.Exec(`
			INSERT INTO providers (id, name, enabled, created_at)
			VALUES (?, ?, 1, ?)
			ON CONFLICT(id) DO NOTHING
		`, p.id, p.name, now)
		if err != nil {
			return fmt.Errorf("seeding provider %s: %w", p.id, err)
		}
	}

	// Seed Public Models
	models := []struct {
		id          string
		displayName string
		description string
	}{
		{"deepseek-v4-flash", "DeepSeek V4 Flash", "Primary balanced DeepSeek V4 Flash model"},
		{"deepseek-v4-flash-fast", "DeepSeek V4 Flash (Fast)", "Low-latency optimized DeepSeek V4 Flash model"},
	}

	for _, m := range models {
		_, err := tx.Exec(`
			INSERT INTO public_models (id, display_name, description, enabled, created_at)
			VALUES (?, ?, ?, 1, ?)
			ON CONFLICT(id) DO NOTHING
		`, m.id, m.displayName, m.description, now)
		if err != nil {
			return fmt.Errorf("seeding public model %s: %w", m.id, err)
		}
	}

	// Seed Provider Model Mappings
	mappings := []struct {
		id            string
		providerID    string
		publicModelID string
		upstreamID    string
		inputRate     int64
		cachedRate    int64
		outputRate    int64
	}{
		// deepseek-v4-flash mappings
		{"map-fw-flash", "fireworks", "deepseek-v4-flash", "accounts/fireworks/models/deepseek-v4-flash", 140000, 14000, 280000},
		{"map-sf-flash", "siliconflow", "deepseek-v4-flash", "deepseek-ai/DeepSeek-V4-Flash", 140000, 14000, 280000},
		{"map-nov-flash", "novita", "deepseek-v4-flash", "deepseek/deepseek-v4-flash", 140000, 14000, 280000},
		{"map-base-flash", "baseten", "deepseek-v4-flash", "deepseek-v4-flash", 140000, 14000, 280000},

		// deepseek-v4-flash-fast mappings
		{"map-base-fast", "baseten", "deepseek-v4-flash-fast", "deepseek-v4-flash", 140000, 14000, 280000},
		{"map-nov-fast", "novita", "deepseek-v4-flash-fast", "deepseek/deepseek-v4-flash", 140000, 14000, 280000},
		{"map-sf-fast", "siliconflow", "deepseek-v4-flash-fast", "deepseek-ai/DeepSeek-V4-Flash", 140000, 14000, 280000},
		{"map-fw-fast", "fireworks", "deepseek-v4-flash-fast", "accounts/fireworks/models/deepseek-v4-flash", 140000, 14000, 280000},
	}

	for _, mp := range mappings {
		_, err := tx.Exec(`
			INSERT INTO provider_model_mappings (
				id, provider_id, public_model_id, upstream_model_id,
				supports_streaming, supports_tools,
				input_rate_per_m_tokens, cached_rate_per_m_tokens, output_rate_per_m_tokens,
				currency, enabled, created_at, updated_at
			) VALUES (?, ?, ?, ?, 1, 1, ?, ?, ?, 'USD', 1, ?, ?)
			ON CONFLICT(id) DO NOTHING
		`, mp.id, mp.providerID, mp.publicModelID, mp.upstreamID, mp.inputRate, mp.cachedRate, mp.outputRate, now, now)
		if err != nil {
			return fmt.Errorf("seeding mapping %s: %w", mp.id, err)
		}
	}

	// Seed Routing Entries
	routesFlash := []struct {
		id        string
		mappingID string
		priority  int
	}{
		{"route-flash-1", "map-fw-flash", 1},
		{"route-flash-2", "map-sf-flash", 2},
		{"route-flash-3", "map-nov-flash", 3},
		{"route-flash-4", "map-base-flash", 4},
	}

	for _, r := range routesFlash {
		_, err := tx.Exec(`
			INSERT INTO routing_entries (id, public_model_id, mapping_id, priority, created_at)
			VALUES (?, 'deepseek-v4-flash', ?, ?, ?)
			ON CONFLICT(public_model_id, mapping_id) DO NOTHING
		`, r.id, r.mappingID, r.priority, now)
		if err != nil {
			return fmt.Errorf("seeding route %s: %w", r.id, err)
		}
	}

	routesFast := []struct {
		id        string
		mappingID string
		priority  int
	}{
		{"route-fast-1", "map-base-fast", 1},
		{"route-fast-2", "map-nov-fast", 2},
		{"route-fast-3", "map-sf-fast", 3},
		{"route-fast-4", "map-fw-fast", 4},
	}

	for _, r := range routesFast {
		_, err := tx.Exec(`
			INSERT INTO routing_entries (id, public_model_id, mapping_id, priority, created_at)
			VALUES (?, 'deepseek-v4-flash-fast', ?, ?, ?)
			ON CONFLICT(public_model_id, mapping_id) DO NOTHING
		`, r.id, r.mappingID, r.priority, now)
		if err != nil {
			return fmt.Errorf("seeding route %s: %w", r.id, err)
		}
	}

	return tx.Commit()
}
