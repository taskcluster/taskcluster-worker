package logprefix

import (
	"fmt"
	"sort"
	"strconv"

	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/plugins"
)

type provider struct {
	plugins.PluginProviderBase
}

type plugin struct {
	plugins.PluginBase
	keys map[string]string
}

func init() {
	plugins.Register("logprefix", provider{})
}

func (provider) ConfigSchema() schematypes.Schema {
	return configSchema
}

func (provider) NewPlugin(options plugins.PluginOptions) (plugins.Plugin, error) {
	keys := make(map[string]string)
	schematypes.MustValidateAndMap(configSchema, options.Config, &keys)

	// Print a neat message to make debugging config easier.
	// Presumably, config files inject stuff into this section using
	// transforms, so it's nice to have some log.
	for k, v := range keys {
		options.Monitor.Infof("Prefixing task logs: '%s' = '%s'", k, v)
	}

	return &plugin{
		keys: keys,
	}, nil
}

func (p *plugin) NewTaskPlugin(options plugins.TaskPluginOptions) (plugins.TaskPlugin, error) {
	keys := map[string]string{
		"TaskId": options.TaskContext.TaskID,
		"RunId":  strconv.Itoa(options.TaskContext.RunID),
	}
	// Construct list of all keys (so we can sort it)
	var allKeys []string
	for k := range keys {
		allKeys = append(allKeys, k)
	}
	for k := range p.keys {
		if !stringContains(allKeys, k) {
			allKeys = append(allKeys, k)
		} else {
			debug("overwriting: %s", k)
		}
	}
	// Sort list of allKeys (to ensure consistency)
	sort.Strings(allKeys)

	// Print keys to task log
	for _, k := range allKeys {
		v, ok := p.keys[k]
		if !ok {
			v = keys[k]
		}

		options.TaskContext.Log(fmt.Sprintf("%s: %s", k, v))
	}

	// Return a plugin that does nothing
	return plugins.TaskPluginBase{}, nil
}

func stringContains(list []string, element string) bool {
	for _, e := range list {
		if e == element {
			return true
		}
	}
	return false
}
