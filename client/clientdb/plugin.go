package clientdb

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/companyzero/bisonrelay/client/clientintf"
)

// PluginExists returns true if the plugin with the specified name exists.
func (db *DB) PluginExists(tx ReadTx, pluginName string) bool {
	fname := filepath.Join(db.root, pluginsDir, pluginName)
	return fileExists(fname)
}

// SavePlugin saves the plugin data to the database.
func (db *DB) SavePlugin(tx ReadWriteTx, plugin Plugin) error {
	dir := filepath.Join(db.root, pluginsDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("unable to make plugins dir: %v", err)
	}

	blob, err := json.Marshal(plugin)
	if err != nil {
		return fmt.Errorf("unable to marshal plugin data: %v", err)
	}

	fname := filepath.Join(dir, plugin.ID)
	if _, err := os.Stat(fname); !os.IsNotExist(err) {
		if err != nil {
			return err
		}
		return fmt.Errorf("plugin %s: %w", plugin.Name, ErrAlreadyExists)
	}
	return os.WriteFile(fname, blob, 0o600)
}

// DeletePlugin deletes the plugin data from the database.
func (db *DB) DeletePlugin(tx ReadWriteTx, pluginName string) error {
	fname := filepath.Join(db.root, pluginsDir, pluginName)
	return os.Remove(fname)
}

// GetPlugin retrieves the plugin data from the database.
func (db *DB) GetPlugin(tx ReadTx, pluginID clientintf.PluginID) (Plugin, error) {
	fname := filepath.Join(db.root, pluginsDir, pluginID.String())
	blob, err := os.ReadFile(fname)
	if err != nil {
		if os.IsNotExist(err) {
			return Plugin{}, fmt.Errorf("plugin %s: %w", pluginID, ErrNotFound)
		}
		return Plugin{}, err
	}

	var plugin Plugin
	err = json.Unmarshal(blob, &plugin)
	if err != nil {
		return Plugin{}, nil
	}
	return plugin, nil
}

// ListPlugins lists all plugins in the database.
func (db *DB) ListPlugins(tx ReadTx) ([]Plugin, error) {
	dir := filepath.Join(db.root, pluginsDir)
	dirEntries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("unable to read plugins dir: %v", err)
	}

	var res []Plugin
	for _, f := range dirEntries {
		if f.IsDir() {
			continue
		}

		fname := filepath.Join(dir, f.Name())
		blob, err := os.ReadFile(fname)
		if err != nil {
			return nil, err
		}

		var plugin Plugin
		err = json.Unmarshal(blob, &plugin)
		if err != nil {
			return nil, fmt.Errorf("unable to unmarshal plugin file %s: %v", fname, err)
		}
		res = append(res, plugin)
	}

	return res, nil
}

// EnablePlugin enables the specified plugin.
func (db *DB) EnablePlugin(tx ReadWriteTx, pluginID clientintf.PluginID) error {
	plugin, err := db.GetPlugin(tx, pluginID)
	if err != nil {
		return err
	}
	plugin.Enabled = true
	plugin.Updated = time.Now()
	return db.SavePlugin(tx, plugin)
}

// DisablePlugin disables the specified plugin.
func (db *DB) DisablePlugin(tx ReadWriteTx, pluginID clientintf.PluginID) error {
	plugin, err := db.GetPlugin(tx, pluginID)
	if err != nil {
		return err
	}
	plugin.Enabled = false
	plugin.Updated = time.Now()
	return db.SavePlugin(tx, plugin)
}

// UpdatePluginConfig updates the configuration of the specified plugin.
func (db *DB) UpdatePluginConfig(tx ReadWriteTx, pluginID clientintf.PluginID, config map[string]interface{}) error {
	plugin, err := db.GetPlugin(tx, pluginID)
	if err != nil {
		return err
	}
	plugin.Config = config
	plugin.Updated = time.Now()
	return db.SavePlugin(tx, plugin)
}
