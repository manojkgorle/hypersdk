// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"fmt"
	"net/url"
	"os"

	"github.com/ava-labs/hypersdk/utils"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

var prometheusCmd = &cobra.Command{
	Use: "prometheus",
	RunE: func(*cobra.Command, []string) error {
		return ErrMissingSubcommand
	},
}

type PrometheusStaticConfig struct {
	Targets []string `yaml:"targets"`
}

type PrometheusScrapeConfig struct {
	JobName       string                    `yaml:"job_name"`
	StaticConfigs []*PrometheusStaticConfig `yaml:"static_configs"`
	MetricsPath   string                    `yaml:"metrics_path"`
}

type PrometheusConfig struct {
	Global struct {
		ScrapeInterval     string `yaml:"scrape_interval"`
		EvaluationInterval string `yaml:"evaluation_interval"`
	} `yaml:"global"`
	ScrapeConfigs []*PrometheusScrapeConfig `yaml:"scrape_configs"`
	Storage       struct {
		Local struct {
			Path string `yaml:"path"`
		} `yaml:"local"`
	} `yaml:"storage"`
}

var generatePrometheusCmd = &cobra.Command{
	Use: "generate",
	RunE: func(_ *cobra.Command, args []string) error {
		// Generate Prometheus-compatible endpoints
		chainID, uris, err := promptChain("select chainID", nil)
		if err != nil {
			return err
		}
		endpoints := make([]string, len(uris))
		for i, uri := range uris {
			host, err := utils.GetHost(uri)
			if err != nil {
				return err
			}
			port, err := utils.GetPort(uri)
			if err != nil {
				return err
			}
			endpoints[i] = fmt.Sprintf("%s:%s", host, port)
		}

		// Create Prometheus YAML
		var prometheusConfig PrometheusConfig
		prometheusConfig.Global.ScrapeInterval = "15s"
		prometheusConfig.Global.EvaluationInterval = "15s"
		prometheusConfig.ScrapeConfigs = []*PrometheusScrapeConfig{
			{
				JobName: "prometheus",
				StaticConfigs: []*PrometheusStaticConfig{
					{
						Targets: endpoints,
					},
				},
				MetricsPath: "/ext/metrics",
			},
		}
		prometheusConfig.Storage.Local.Path = prometheusData
		yamlData, err := yaml.Marshal(&prometheusConfig)
		if err != nil {
			return err
		}
		if err := os.WriteFile(prometheusFile, yamlData, fsModeWrite); err != nil {
			return err
		}
		utils.Outf("{{green}}prometheus config file created:{{/}} %s\n", prometheusFile)
		utils.Outf("{{green}}prometheus data directory created:{{/}} %s\n", prometheusData)

		// Log useful queries
		panels := []string{}
		panels = append(panels, fmt.Sprintf("avalanche_%s_blks_processing", chainID))
		utils.Outf("{{yellow}}blocks processing:{{/}} %s\n", panels[len(panels)-1])
		panels = append(panels, fmt.Sprintf("avalanche_%s_blks_accepted_count", chainID))
		utils.Outf("{{yellow}}blocks accepted:{{/}} %s\n", panels[len(panels)-1])
		panels = append(panels, fmt.Sprintf("avalanche_%s_blks_rejected_count", chainID))
		utils.Outf("{{yellow}}blocks rejected:{{/}} %s\n", panels[len(panels)-1])
		panels = append(panels, fmt.Sprintf("increase(avalanche_%s_vm_hyper_sdk_vm_txs_accepted[30s])/30", chainID))
		utils.Outf("{{yellow}}transactions per second:{{/}} %s\n", panels[len(panels)-1])
		panels = append(panels, fmt.Sprintf("avalanche_%s_vm_hyper_sdk_chain_mempool_size", chainID))
		utils.Outf("{{yellow}}mempool size:{{/}} %s\n", panels[len(panels)-1])
		panels = append(panels, "avalanche_resource_tracker_cpu_usage")
		utils.Outf("{{yellow}CPU usage:{{/}} %s\n", panels[len(panels)-1])
		panels = append(panels, fmt.Sprintf("increase(avalanche_%s_handler_chits_sum[30s])/1000000/30 + increase(avalanche_%s_handler_notify_sum[30s])/1000000/30 + increase(avalanche_%s_handler_get_sum[30s])/1000000/30 + increase(avalanche_%s_handler_push_query_sum[30s])/1000000/30 + increase(avalanche_%s_handler_put_sum[30s])/1000000/30 + increase(avalanche_%s_handler_pull_query_sum[30s])/1000000/30 + increase(avalanche_%s_handler_query_failed_sum[30s])/1000000/30", chainID, chainID, chainID, chainID, chainID, chainID, chainID))
		utils.Outf("{{yellow}}consensus engine processing (ms/s):{{/}} %s\n", panels[len(panels)-1])

		// Generated dashboard link
		dashboard := "http://localhost:9090/graph?"
		params := url.Values{}
		for i, panel := range panels {
			params.Add(fmt.Sprintf("g%d.expr", i), panel)
			params.Add(fmt.Sprintf("g%d.tab", i), "0")
		}
		// http://localhost:9090/graph?g0.expr=avalanche_K3XqAiCksoL5HSAsYECY89DCdLGH3sJLM2GLWyGVmNss5yCPw_blks_processing&g0.tab=1&g0.stacked=0&g0.show_exemplars=0&g0.range_input=1h
		utils.Outf("{{orange}}pre-built dashboard:{{/}} %s%s\n", dashboard, params.Encode())
		return nil
	},
}
