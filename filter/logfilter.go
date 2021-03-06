package filter

import (
	"encoding/json"
	"io"
	"log"
	"regexp"
	"strings"
)

var (
	blacklistedProperties = []string{
		"_GID",
		"_CAP_EFFECTIVE",
		"SYSLOG_FACILITY",
		"PRIORITY",
		"SYSLOG_IDENTIFIER",
		"_BOOT_ID",
		"_CMDLINE",
		"_COMM",
		"_EXE",
		"_SYSTEMD_CGROUP",
		"_SYSTEMD_SLICE",
		"_TRANSPORT",
		"_UID",
		"__CURSOR",
		"__MONOTONIC_TIMESTAMP",
		"_SELINUX_CONTEXT",
		"__REALTIME_TIMESTAMP",
		"_PID",
		"CONTAINER_ID",
		"CONTAINER_ID_FULL",
		"CONTAINER_NAME",
		"CONTAINER_TAG",
		"MACHINE_ID",
		"_SOURCE_REALTIME_TIMESTAMP",
		"_SYSTEMD_INVOCATION_ID",
	}

	blacklistedUnits = map[string]bool{
		"log-collector.service":      true,
		"logstash-forwarder.service": true,
		"flanneld.service":           true,
	}

	blacklistedServices = map[string]bool{
		"main":                           true,
		"cluster-autoscaler":             true,
		"kube-resources-autosave-pusher": true,
		"kube-resources-autosave-dumper": true,
		"resilient-splunk-forwarder":     true,
		"coco-diamond":                   true,
	}

	blacklistedStrings = []string{
		"transaction_id=SYNTHETIC-REQ",
		`"transaction_id":"SYNTHETIC-REQ`,
		"__health",
		"__gtg",

		// this is extensively logged by the kubelet.service when mounting the volume
		// holding the default token for the service account.
		"MountVolume.SetUp succeeded for volume",
	}

	blacklistedSyslogIds = map[string]bool{
		"dockerd": true,
	}

	blacklistedContainerTags = []string{
		"gcr.io/google_containers/heapster",
		"gcr.io/google_containers/kubedns-amd64",
		"gcr.io/google_containers/addon-resizer",
	}

	propertyMapping = map[string]string{
		"_SYSTEMD_UNIT": "SYSTEMD_UNIT",
		"_MACHINE_ID":   "MACHINE_ID",
		"_HOSTNAME":     "HOSTNAME",
	}
)

var (
	Env        string
	DNSAddress string
	mc         clusterService
)

// Filters & enhances the JSON log messages that come into the reader, and writes the resulted log messages to the writer.
func Filter(r io.Reader, w io.Writer) {
	defer log.Println("Log filter completed")

	mc = newMonitoredClusterService(DNSAddress, Env)

	dec := json.NewDecoder(r)
	enc := json.NewEncoder(w)
	for {
		m := make(map[string]interface{})
		err := dec.Decode(&m)
		if err != nil {
			if err == io.EOF {
				return
			}
			panic(err)
		}
		keep := processMessage(m)
		if keep {
			enc.Encode(m)
		}
	}
}

func processMessage(m map[string]interface{}) bool {
	unit := m["_SYSTEMD_UNIT"]
	if unitString, ok := unit.(string); ok {
		if blacklistedUnits[unitString] {
			return false
		}
	}

	serviceName := computeServiceName(m)
	if blacklistedServices[serviceName] {
		return false
	}

	syslogID := m["SYSLOG_IDENTIFIER"]
	if syslogIDString, ok := syslogID.(string); ok {
		if blacklistedSyslogIds[syslogIDString] {
			return false
		}
	}

	containerTag := m["CONTAINER_TAG"]
	if containerTagString, ok := containerTag.(string); ok {
		if containsBlacklistedString(containerTagString, blacklistedContainerTags) {
			return false
		}
	}

	message := fixBytesToString(m["MESSAGE"]).(string)

	if containsBlacklistedString(message, blacklistedStrings) {
		return false
	}

	message = hideAPIKeysInURLQueryParams(message)

	munge(m, message)
	removeBlacklistedProperties(m)
	renameProperties(m)
	return true
}

func containsBlacklistedString(message string, blacklistedStrings []string) bool {
	for _, blacklistedString := range blacklistedStrings {
		if strings.Contains(message, blacklistedString) {
			return true
		}
	}
	return false
}

var apiKeyQueryParamRegExp = regexp.MustCompile("(?i)api_?key(?-i)=[^\\s&]+")

func hideAPIKeysInURLQueryParams(msg string) string {
	queryParams := apiKeyQueryParamRegExp.FindAllString(msg, -1)
	for _, queryParam := range queryParams {
		splitParam := strings.Split(queryParam, "=")
		paramName := splitParam[0]
		key := splitParam[1]
		obscuredKey := key[:len(key)/2] + strings.Repeat("*", 8)
		msg = strings.Replace(msg, queryParam, paramName+"="+obscuredKey, 1)
	}
	return msg
}

func munge(m map[string]interface{}, message string) {

	m["platform"] = "up-k8s"
	if Env != "" {
		m["environment"] = Env
	}

	podName := extractPodName(m["CONTAINER_NAME"])
	if podName != "" {
		m["POD_NAME"] = podName
	}

	message = fixNewLines(message)
	m["MESSAGE"] = message

	tid := extractTransactionID(message)
	if tid != "" {
		m["transaction_id"] = tid
	}

	ent, ok, format := extract(message)
	if !ok {
		return
	}

	// hackity
	j, err := json.Marshal(ent)
	if err != nil {
		panic(err)
	}
	entMap := make(map[string]interface{})
	err = json.Unmarshal(j, &entMap)
	if err != nil {
		panic(err)
	}
	for k, v := range entMap {
		m[k] = v
	}

	//avoid field duplication
	if format == "json" {
		delete(m, "MESSAGE")
	}

	if m["monitoring_event"] == "true" {
		m["active_cluster"], _ = mc.isActive()
	}
}

func computeServiceName(m map[string]interface{}) string {
	serviceName := extractServiceName(m["CONTAINER_NAME"])
	if serviceName != "" && serviceName != "POD" {
		m["SERVICE_NAME"] = serviceName
	}
	return serviceName
}

func extractServiceName(containerTag interface{}) string {
	containerNameSplitByUnderscores := splitByUnderscores(containerTag)

	if len(containerNameSplitByUnderscores) >= 1 {
		stringArray := strings.Split(containerNameSplitByUnderscores[1], ".")
		return stringArray[0]
	}

	return ""
}

func extractPodName(containerTag interface{}) string {
	containerNameSplitByUnderscores := splitByUnderscores(containerTag)

	if len(containerNameSplitByUnderscores) > 2 {
		return containerNameSplitByUnderscores[2]
	}

	return ""
}

func splitByUnderscores(i interface{}) []string {
	if s, ok := i.(string); ok {
		items := strings.Split(s, "_")
		return items
	}

	return []string{}
}

var tidRegex = regexp.MustCompile(`\btransaction_id=([A-Za-z0-9\-_:]+)`)

func extractTransactionID(message string) string {
	matches := tidRegex.FindAllStringSubmatch(message, -1)
	if len(matches) != 0 {
		return matches[0][1]
	}

	return ""
}

// workaround for cases where a string has been turned into a
// byte array, or more accurately an array of float64, since
// we've been via json.
// TODO: remove this hack once the underlying cause is found
func fixBytesToString(message interface{}) interface{} {
	intArray, ok := message.([]interface{})
	if !ok {
		return message
	}

	data := make([]byte, len(intArray))
	for i, v := range intArray {
		f64, ok := v.(float64)
		if !ok {
			return message
		}
		data[i] = byte(f64)
	}
	return string(data)
}

func fixNewLines(message string) string {
	return strings.Replace(message, "|", "\n", -1)
}

func removeBlacklistedProperties(m map[string]interface{}) {
	for _, p := range blacklistedProperties {
		delete(m, p)
	}
}

func renameProperties(m map[string]interface{}) {
	for p, r := range propertyMapping {
		value := m[p]
		if value != nil {
			delete(m, p)
			m[r] = value
		}
	}

}
