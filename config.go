package main

type OIDTranslation struct {
	SourceOID       string `toml:"source_oid"`
	TargetOID       string `toml:"target_oid"`
	TranslationType string `toml:"translation_type"`
}

type Config struct {
	TargetIP           string           `toml:"target_ip"`
	SourceIP           string           `toml:"source_ip"`
	ProxyLkIP          string           `toml:"proxy_lk_ip"`
	RequestPort        int              `toml:"request_port"`
	TrapPort           int              `toml:"trap_port"`
	HeartbeatIP        string           `toml:"heartbeat_ip"`
	HeartbeatCommunity string           `toml:"heartbeat_community"`
	HeartbeatInterval  int              `toml:"heartbeat_interval"`
	Oids               []OIDTranslation `toml:"oids"`
}
