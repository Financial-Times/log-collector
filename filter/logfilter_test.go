package filter

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFixBytesToString(t *testing.T) {
	// happy path
	input := []interface{}{float64('A'), float64('B')}
	output := fixBytesToString(input)
	expected := "AB"
	if output != expected {
		t.Errorf("expected output %v but got %v\n", expected, output)
	}
}

const expectedNewlines = `A
B
C`

func TestFixNewlines(t *testing.T) {
	input := "A|B|C"
	output := fixNewLines(input)
	if output != expectedNewlines {
		t.Errorf("expected %v but got %v\n", expectedNewlines, output)
	}
}

var rawJSON = map[string]interface{}{
	"MESSAGE":               "message",
	"_HOSTNAME":             "hostname",
	"_MACHINE_ID":           "machine",
	"_SYSTEMD_UNIT":         "system",
	"_GID":                  "gid",
	"_COMM":                 "comm",
	"_EXE":                  "exe",
	"_CAP_EFFECTIVE":        "cap",
	"SYSLOG_FACILITY":       "syslog",
	"PRIORITY":              "priority",
	"SYSLOG_IDENTIFIER":     "syslogi",
	"_BOOT_ID":              "boot",
	"_CMDLINE":              "cmd",
	"_SYSTEMD_CGROUP":       "cgroup",
	"_SYSTEMD_SLICE":        "slice",
	"_TRANSPORT":            "transport",
	"_UID":                  "uid",
	"__CURSOR":              "cursor",
	"__MONOTONIC_TIMESTAMP": "monotonic",
	"_PID":                  "pid",
	"_SELINUX_CONTEXT":      "selinux context",
	"__REALTIME_TIMESTAMP":  "realtime timestamp",
}

var blacklistFilteredJSON = map[string]interface{}{
	"MESSAGE":       "message",
	"_HOSTNAME":     "hostname",
	"_MACHINE_ID":   "machine",
	"_SYSTEMD_UNIT": "system",
}

var blacklistFilteredAndPropertiesRenamedJSON = map[string]interface{}{
	"MESSAGE":      "message",
	"HOSTNAME":     "hostname",
	"MACHINE_ID":   "machine",
	"SYSTEMD_UNIT": "system",
}

func TestApplyPropertyBlacklist(t *testing.T) {
	removeBlacklistedProperties(rawJSON)
	if !reflect.DeepEqual(rawJSON, blacklistFilteredJSON) {
		t.Errorf("expected %v but got %v\n", blacklistFilteredJSON, rawJSON)
	}
}

func TestShouldRenameProperties(t *testing.T) {
	renameProperties(blacklistFilteredJSON)
	if !reflect.DeepEqual(blacklistFilteredJSON, blacklistFilteredAndPropertiesRenamedJSON) {
		t.Errorf("expected %v but got %v\n", blacklistFilteredAndPropertiesRenamedJSON, blacklistFilteredJSON)
	}
}

func TestEnvTag(t *testing.T) {
	Env = ""

	m := make(map[string]interface{})
	munge(m, "")

	if m["environment"] != nil {
		t.Errorf("didn't expect to find environment %v", m["environment"])
	}

	Env = "foo"

	munge(m, "")

	if m["environment"] != "foo" {
		t.Errorf("expected foo but got  %v", m["environment"])
	}

}

func TestTransactionId(t *testing.T) {
	testCases := []struct {
		name     string
		message  string
		expected string
	}{
		{
			name:     "standard API call",
			message:  "foo baz baz transaction_id=transid_a-b banana",
			expected: "transid_a-b",
		},
		{
			name:     "message without transaction id",
			message:  "foo baz baz transzzzaction_id=transid_a-b banana",
			expected: "",
		},
		{
			name:     "PAM notifications feed transaction id may contain colon character",
			message:  "INFO  [2017-01-19 12:05:13,478] com.ft.api.util.transactionid.TransactionIdFilter: transaction_id=tid_pam_notifications_pull_2017-01-19T12:05:13Z [REQUEST HANDLED] uri=/content/notifications time_ms=2 status=200 exception_was_thrown=false [dw-1968]",
			expected: "tid_pam_notifications_pull_2017-01-19T12:05:13Z",
		},
		{
			name:     "transaction_id should not include parenthesis or quotes",
			message:  "foo baz baz \"My User Agent (transaction_id=transid_a-b)\" banana",
			expected: "transid_a-b",
		},
	}

	for _, c := range testCases {
		m := map[string]interface{}{
			"MESSAGE": c.message,
		}
		munge(m, c.message)

		actual, found := m["transaction_id"]
		if len(c.expected) == 0 {
			assert.False(t, found, fmt.Sprintf("expected no transaction_id for %s", c.name))
		} else {
			assert.Equal(t, c.expected, actual, fmt.Sprintf("transaction_id for %s", c.name))
		}
	}
}

func TestContainsBlacklistedStringWithBlacklistedString(t *testing.T) {
	message := "foo baz baz " + blacklistedStrings[0] + " foo "

	if !containsBlacklistedString(message, blacklistedStrings) {
		t.Error("Expected to detect blacklisted string in test")
	}

}

func TestContainsBlacklistedStringWithoutBlacklistedString(t *testing.T) {
	message := "foo baz baz transazzzction_id=transid_a-b banana"

	if containsBlacklistedString(message, blacklistedStrings) {
		t.Error("Detected black listed string when there was none")
	}

}

func TestBlacklistedServices(t *testing.T) {
	for blacklistedService := range blacklistedServices {
		msg := msgWithContainerName(blacklistedService)
		m := make(map[string]interface{})
		_ = json.Unmarshal([]byte(msg), &m)
		assert.False(t, processMessage(m))
	}
}

func TestNotBlacklistedServices(t *testing.T) {
	testCases := []struct {
		jsonString string
		processed  bool
	}{
		{
			jsonString: msgWithContainerName("publish-availability-monitor"),
			processed:  true,
		},
	}

	for _, c := range testCases {
		m := make(map[string]interface{})
		_ = json.Unmarshal([]byte(c.jsonString), &m)
		ok := processMessage(m)
		assert.True(t, c.processed == ok)
	}
}

func TestBlacklistedContainerTags(t *testing.T) {
	for _, blacklistedTag := range blacklistedContainerTags {
		msg := msgWithContainerTag(blacklistedTag)
		m := make(map[string]interface{})
		_ = json.Unmarshal([]byte(msg), &m)
		assert.False(t, processMessage(m))
	}
}

func TestNotBlacklistedContainerTag(t *testing.T) {
	testCases := []struct {
		jsonString string
		processed  bool
	}{
		{
			jsonString: msgWithContainerTag("publish-availability-monitor"),
			processed:  true,
		},
	}

	for _, c := range testCases {
		m := make(map[string]interface{})
		_ = json.Unmarshal([]byte(c.jsonString), &m)
		ok := processMessage(m)
		assert.True(t, c.processed == ok)
	}
}

//TODO This needs to be properly fixed for kubernetes clusters
func TestClusterStatus(t *testing.T) {
	trueVar := true
	falseVar := false

	testCases := []struct {
		jsonString string
		dnsAddress string
		tag        string
		expected   *bool
	}{
		{
			jsonString: `{"@time":"2017-09-12T14:19:28.199162596Z","HOSTNAME":"ip-172-24-159-194.eu-west-1.compute.internal","MACHINE_ID":"1234","MESSAGE":"{\"@time\":\"2017-09-12T14:19:28.199162596Z\",\"content_type\":\"Suggestions\",\"event\":\"SaveNeo4j\",\"level\":\"info\",\"monitoring_event\":\"true\",\"msg\":\"%s successfully written in Neo4jSuggestions\",\"service_name\":\"suggestions-rw-neo4j\",\"transaction_id\":\"tid_u7pkkludzd\",\"uuid\":\"0ec3c76b-9be4-4d76-b1f9-5414460a8bc1\"}","SYSTEMD_UNIT":"suggestions-rw-neo4j@1.service","_SYSTEMD_INVOCATION_ID":"1234","content_type":"Suggestions","environment":"xp","event":"SaveNeo4j","level":"info","monitoring_event":"true","msg":"%s successfully written in Neo4jSuggestions","platform":"up-coco","service_name":"suggestions-rw-neo4j","transaction_id":"tid_test","uuid":"a3f63cda-97af-11e7-b83c-9588e51488a0"}`,
			dnsAddress: "google.com",
			tag:        "go",
			expected:   &trueVar,
		},
		{
			jsonString: `{"@time":"2017-09-12T14:19:28.199162596Z","HOSTNAME":"ip-172-24-159-194.eu-west-1.compute.internal","MACHINE_ID":"1234","MESSAGE":"{\"@time\":\"2017-09-12T14:19:28.199162596Z\",\"content_type\":\"Suggestions\",\"event\":\"SaveNeo4j\",\"level\":\"info\",\"monitoring_event\":\"true\",\"msg\":\"%s successfully written in Neo4jSuggestions\",\"service_name\":\"suggestions-rw-neo4j\",\"transaction_id\":\"tid_u7pkkludzd\",\"uuid\":\"0ec3c76b-9be4-4d76-b1f9-5414460a8bc1\"}","SYSTEMD_UNIT":"suggestions-rw-neo4j@1.service","_SYSTEMD_INVOCATION_ID":"1234","content_type":"Suggestions","environment":"xp","event":"SaveNeo4j","level":"info","monitoring_event":"true","msg":"%s successfully written in Neo4jSuggestions","platform":"up-coco","service_name":"suggestions-rw-neo4j","transaction_id":"tid_test","uuid":"a3f63cda-97af-11e7-b83c-9588e51488a0"}`,
			dnsAddress: "google.com",
			tag:        "invalid",
			expected:   &falseVar,
		},
		{
			jsonString: `{"@time":"2017-09-12T14:19:28.199162596Z","HOSTNAME":"ip-172-24-159-194.eu-west-1.compute.internal","MACHINE_ID":"1234","MESSAGE":"{\"@time\":\"2017-09-12T14:19:28.199162596Z\",\"content_type\":\"Suggestions\",\"event\":\"SaveNeo4j\",\"level\":\"info\",\"msg\":\"%s successfully written in Neo4jSuggestions\",\"service_name\":\"suggestions-rw-neo4j\",\"transaction_id\":\"tid_u7pkkludzd\",\"uuid\":\"a0ec3c76b-9be4-4d76-b1f9-5414460a8bc1\"}","SYSTEMD_UNIT":"suggestions-rw-neo4j@1.service","_SYSTEMD_INVOCATION_ID":"1234","content_type":"Suggestions","environment":"xp","event":"SaveNeo4j","level":"info","msg":"%s successfully written in Neo4jSuggestions","platform":"up-coco","service_name":"suggestions-rw-neo4j","transaction_id":"tid_test","uuid":"a3f63cda-97af-11e7-b83c-9588e51488a0"}`,
			dnsAddress: "google.com",
			tag:        "go",
			expected:   nil,
		},
	}

	for _, c := range testCases {
		mc = newMonitoredClusterService(c.dnsAddress, c.tag)
		m := make(map[string]interface{})
		_ = json.NewDecoder(strings.NewReader(c.jsonString)).Decode(&m)
		processMessage(m)
		if c.expected == nil {
			assert.Nil(t, m["active_cluster"])
		} else {
			assert.Equal(t, *c.expected, m["active_cluster"])
		}
	}
}

func TestExtractPodNameWithEmptyContainerTag(t *testing.T) {
	if podName := extractPodName(""); podName != "" {
		t.Error("Expected empty string as pod name when empty container tag is provided")
	}
}

func TestExtractPodNameWithNonStringContainerTag(t *testing.T) {
	nonStringContainerTag := 1
	if podName := extractPodName(nonStringContainerTag); podName != "" {
		t.Error("Expected empty string as pod name when non string container tag is provided")
	}
}

func TestExtractPodNameWithContainerTagWithoutUnderscores(t *testing.T) {
	if podName := extractPodName("test"); podName != "" {
		t.Error("Expected empty string as pod name when container tag without underscores is provided")
	}
}

func TestExtractPodNameWithContainerTagWithOneUnderscore(t *testing.T) {
	if podName := extractPodName("test_a"); podName != "" {
		t.Error("Expected empty string as pod name when container tag with one underscore is provided")
	}
}

func TestExtractPodNameWithValidContainerTagContainingTwoUnderscores(t *testing.T) {
	if podName := extractPodName("test_a_b"); podName != "b" {
		t.Error("Expected non empty string as pod name when container tag with two underscores is provided")
	}
}

func TestExtractPodNameWithValidContainerTagContainingMoreThanTwoUnderscores(t *testing.T) {
	if podName := extractPodName("test_a_b_c"); podName != "b" {
		t.Error("Expected third substring from container tag as pod name when container tag with more two underscores is provided")
	}
}

func TestHideSingleAPIKeysInURLQueryParam(t *testing.T) {
	msgWithAPIKey := `10.2.26.0 ops-17-01-2018 30/Jan/2018:08:35:04 /content/notifications-push?apiKey=vhs2aazf3gyywm3wk2sv44wb&type=ALL 200 -2147483648 "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.132 Safari/537.36" transaction_id=- miss`
	expectedMsg := `10.2.26.0 ops-17-01-2018 30/Jan/2018:08:35:04 /content/notifications-push?apiKey=vhs2aazf3gyy********&type=ALL 200 -2147483648 "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.132 Safari/537.36" transaction_id=- miss`
	actualMsg := hideAPIKeysInURLQueryParams(msgWithAPIKey)
	assert.Equal(t, expectedMsg, actualMsg)
}

func TestHideMultipleAPIKeysInURLQueryParams(t *testing.T) {
	msgWithAPIKey := `10.2.26.0 ops-17-01-2018 30/Jan/2018:08:35:04 /content/notifications-push?apiKey=vhs2aazf3gyywm3wk2sv44wb&type=ALL /content/notifications-push?api_key=wm3wk2sv44wbvhs2aazf3gyy`
	expectedMsg := `10.2.26.0 ops-17-01-2018 30/Jan/2018:08:35:04 /content/notifications-push?apiKey=vhs2aazf3gyy********&type=ALL /content/notifications-push?api_key=wm3wk2sv44wb********`
	actualMsg := hideAPIKeysInURLQueryParams(msgWithAPIKey)
	assert.Equal(t, expectedMsg, actualMsg)
}

func TestBypassWithoutAPIKeysInURLQueryParams(t *testing.T) {
	msgWithoutAPIKey := `10.2.26.0 ops-17-01-2018 30/Jan/2018:08:35:04 /content/notifications-push?type=ALL 200 -2147483648 "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.132 Safari/537.36" transaction_id=- miss`
	expectedMsg := `10.2.26.0 ops-17-01-2018 30/Jan/2018:08:35:04 /content/notifications-push?type=ALL 200 -2147483648 "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.132 Safari/537.36" transaction_id=- miss`
	actualMsg := hideAPIKeysInURLQueryParams(msgWithoutAPIKey)
	assert.Equal(t, expectedMsg, actualMsg)
}

func TestExtractTransactionID(t *testing.T) {
	for _, c := range tidTestCases {
		actualTID := extractTransactionID(c.message)
		assert.Equal(t, c.expectedTID, actualTID, fmt.Sprintf("%s not extracted correctly ", c.name))
	}
}

func BenchmarkExtractTransactionID(b *testing.B) {
	for n := 0; n < b.N; n++ {
		for _, c := range tidTestCases {
			extractTransactionID(c.message)
		}
	}
}

func BenchmarkProcessMessage(b *testing.B) {
	for n := 0; n < b.N; n++ {
		b.StopTimer()
		input := unprocessedMessages()
		b.StartTimer()
		for _, m := range input {
			processMessage(m)
		}
	}
}

func BenchmarkExtractServiceName(b *testing.B) {
	for n := 0; n < b.N; n++ {
		for containerName := range containerNames {
			extractServiceName(containerName)
		}
	}
}

func BenchmarkMunge(b *testing.B) {
	for n := 0; n < b.N; n++ {
		for _, message := range processedMessages {
			m := map[string]interface{}{
				"MESSAGE":        message,
				"CONTAINER_NAME": containerNames[0],
			}
			munge(m, message)
		}
	}
}

func BenchmarkContainsBlacklistedString(b *testing.B) {
	for n := 0; n < b.N; n++ {
		for _, m := range processedMessages {
			containsBlacklistedString(m, blacklistedStrings)
		}
	}
}

func BenchmarkHideSingleAPIKeysInURLQueryParam(b *testing.B) {
	for n := 0; n < b.N; n++ {
		for _, m := range messagesWithApiKeys {
			hideAPIKeysInURLQueryParams(m)
		}
	}
}

func msgWithContainerName(service string) string {
	return fmt.Sprintf(`{"CONTAINER_ID":"03d1f4078733","CONTAINER_ID_FULL":"03d1f4078733f75f4505b07d1f8a3e8287ed497d9d54e0e785440cb969378ca3","CONTAINER_NAME":"k8s_%s_%s-79d574774-2rxrj_kube-system_a093cbca-fb5a-11e7-a6b6-06263dd4a414_6","CONTAINER_TAG":"gcr.io/google_containers/cluster-autoscaler@sha256:6ceb111a36020dc2124c0d7e3746088c20c7e3806a1075dd9e5fe1c42f744fff","HOSTNAME":"ip-10-172-40-164.eu-west-1.compute.internal","MACHINE_ID":"8d1225f40ee64cc7bcce2f549a41657c","MESSAGE":"I0119 15:38:05.932385 1 leaderelection.go:199] successfully renewed lease kube-system/cluster-autoscaler","POD_NAME":"cluster-autoscaler-79d574774-2rxrj","SYSTEMD_UNIT":"docker.service","_SOURCE_REALTIME_TIMESTAMP":"1516376285932645","_SYSTEMD_INVOCATION_ID":"e3b2703c430f45e8a7075dbcf6b3a588","environment":"upp-prod-publish-eu","platform":"up-coco"}`, service, service)
}

func msgWithContainerTag(containerTag string) string {
	return fmt.Sprintf(`{"CONTAINER_ID":"03d1f4078733","CONTAINER_ID_FULL":"03d1f4078733f75f4505b07d1f8a3e8287ed497d9d54e0e785440cb969378ca3","CONTAINER_NAME":"k8s_not-black_not-black-79d574774-2rxrj_kube-system_a093cbca-fb5a-11e7-a6b6-06263dd4a414_6","CONTAINER_TAG":"%s@sha256:6ceb111a36020dc2124c0d7e3746088c20c7e3806a1075dd9e5fe1c42f744fff","HOSTNAME":"ip-10-172-40-164.eu-west-1.compute.internal","MACHINE_ID":"8d1225f40ee64cc7bcce2f549a41657c","MESSAGE":"I0119 15:38:05.932385 1 leaderelection.go:199] successfully renewed lease kube-system/cluster-autoscaler","POD_NAME":"cluster-autoscaler-79d574774-2rxrj","SERVICE_NAME":"whatever-api","SYSTEMD_UNIT":"docker.service","_SOURCE_REALTIME_TIMESTAMP":"1516376285932645","_SYSTEMD_INVOCATION_ID":"e3b2703c430f45e8a7075dbcf6b3a588","environment":"upp-prod-publish-eu","platform":"up-coco"}`, containerTag)
}

var tidTestCases = []struct {
	name        string
	expectedTID string
	message     string
}{
	{
		name:        "uuid style tid",
		expectedTID: "FD7C5F29-6BE7-479A-939F-D42A41CBFD6F",
		message:     `{"CONTAINER_ID":"03d1f4078733","CONTAINER_ID_FULL":"03d1f4078733f75f4505b07d1f8a3e8287ed497d9d54e0e785440cb969378ca3","CONTAINER_NAME":"k8s_not-black_not-black-79d574774-2rxrj_kube-system_a093cbca-fb5a-11e7-a6b6-06263dd4a414_6","CONTAINER_TAG":"%s@sha256:6ceb111a36020dc2124c0d7e3746088c20c7e3806a1075dd9e5fe1c42f744fff","HOSTNAME":"ip-127-0-0-1.north-1.compute.internal","MACHINE_ID":"8d1225f40ee64cc7bcce2f549a41657c","MESSAGE":INFO  [2019-04-09 09:23:06,768] com.ft.enrichedcontent.service.RelationsService: transaction_id=FD7C5F29-6BE7-479A-939F-D42A41CBFD6F Error 404 in relations for UUID 393d1aa6-1866-11e9-b93e-f4351a53f1c3, returning empty map.[pool-12-thread-88130]""POD_NAME":"cluster-autoscaler-79d574774-2rxrj", "SERVICE_NAME":"whatever-api", "SYSTEMD_UNIT":"docker.service", "_SOURCE_REALTIME_TIMESTAMP":"1516376285932645", "_SYSTEMD_INVOCATION_ID":"e3b2703c430f45e8a7075dbcf6b3a588", "environment":"upp-prod-publish-eu", "platform":"up-coco"`,
	},
	{
		name:        "empty tid",
		expectedTID: "-",
		message:     `CONTAINER_ID":"03d1f4078733","CONTAINER_ID_FULL":"03d1f4078733f75f4505b07d1f8a3e8287ed497d9d54e0e785440cb969378ca3","CONTAINER_NAME":"k8s_not-black_not-black-79d574774-2rxrj_kube-system_a093cbca-fb5a-11e7-a6b6-06263dd4a414_6","CONTAINER_TAG":"%s@sha256:6ceb111a36020dc2124c0d7e3746088c20c7e3806a1075dd9e5fe1c42f744fff","HOSTNAME":"ip-127-0-0-1.north-1.compute.internal","MACHINE_ID":"8d1225f40ee64cc7bcce2f549a41657c","MESSAGE":"34.193.167.29, 10.2.0.0 upp-publishing-prod 09/Apr/2019:09:23:01 /__kafka-rest-proxy/consumers/kafka-bridge-pub-prod-us-staging-us/instances/rest-consumer-1-1140/topics/PreNativeCmsPublicationEvents - 1009824 ""Go-http-client/1.1"" transaction_id=- pipe"POD_NAME":"cluster-autoscaler-79d574774-2rxrj","SERVICE_NAME":"whatever-api","SYSTEMD_UNIT":"docker.service","_SOURCE_REALTIME_TIMESTAMP":"1516376285932645","_SYSTEMD_INVOCATION_ID":"e3b2703c430f45e8a7075dbcf6b3a588","environment":"upp-prod-publish-eu","platform":"up-coco"}`,
	},
	{
		name:        "pam pull notifications tid",
		expectedTID: "tid_pam_notifications_pull_2019-04-09T09:23:06Z",
		message:     `"CONTAINER_ID":"03d1f4078733","CONTAINER_ID_FULL":"03d1f4078733f75f4505b07d1f8a3e8287ed497d9d54e0e785440cb969378ca3","CONTAINER_NAME":"k8s_not-black_not-black-79d574774-2rxrj_kube-system_a093cbca-fb5a-11e7-a6b6-06263dd4a414_6","CONTAINER_TAG":"%s@sha256:6ceb111a36020dc2124c0d7e3746088c20c7e3806a1075dd9e5fe1c42f744fff","HOSTNAME":"ip-127-0-0-1.north-1.compute.internal","MACHINE_ID":"8d1225f40ee64cc7bcce2f549a41657c","MESSAGE":"52.30.14.145, 10.2.8.0 ops-16-11-2017 09/Apr/2019:09:23:06 /__list-notifications-rw/lists/notifications?since=2019-04-09T09%3A19%3A20.468Z - 6016 ""UPP Publish Availability Monitor"" transaction_id=tid_pam_notifications_pull_2019-04-09T09:23:06Z pipe"POD_NAME":"cluster-autoscaler-79d574774-2rxrj","SERVICE_NAME":"whatever-api","SYSTEMD_UNIT":"docker.service","_SOURCE_REALTIME_TIMESTAMP":"1516376285932645","_SYSTEMD_INVOCATION_ID":"e3b2703c430f45e8a7075dbcf6b3a588","environment":"upp-prod-publish-eu","platform":"up-coco"}`,
	},
	{
		name:        "pam tid",
		expectedTID: "tid_pam_7wcgp00tcy",
		message:     `"CONTAINER_ID":"03d1f4078733","CONTAINER_ID_FULL":"03d1f4078733f75f4505b07d1f8a3e8287ed497d9d54e0e785440cb969378ca3","CONTAINER_NAME":"k8s_not-black_not-black-79d574774-2rxrj_kube-system_a093cbca-fb5a-11e7-a6b6-06263dd4a414_6","CONTAINER_TAG":"%s@sha256:6ceb111a36020dc2124c0d7e3746088c20c7e3806a1075dd9e5fe1c42f744fff","HOSTNAME":"ip-127-0-0-1.north-1.compute.internal","MACHINE_ID":"8d1225f40ee64cc7bcce2f549a41657c","MESSAGE":"52.30.14.145 - - [09/Apr/2019:09:20:44 +0000] ""GET /content/b4fb99d6-5aa8-11e9-9dde-7aedca0a081a HTTP/1.1"" 404 43 ""-"" ""UPP Publish Availability Monitor"" 1 transaction_id=tid_pam_7wcgp00tcy"POD_NAME":"cluster-autoscaler-79d574774-2rxrj","SERVICE_NAME":"whatever-api","SYSTEMD_UNIT":"docker.service","_SOURCE_REALTIME_TIMESTAMP":"1516376285932645","_SYSTEMD_INVOCATION_ID":"e3b2703c430f45e8a7075dbcf6b3a588","environment":"upp-prod-publish-eu","platform":"up-coco"}`,
	},
	{
		name:        "republish tid",
		expectedTID: "republish_tid_PFuz36ssPr",
		message:     `"CONTAINER_ID":"03d1f4078733","CONTAINER_ID_FULL":"03d1f4078733f75f4505b07d1f8a3e8287ed497d9d54e0e785440cb969378ca3","CONTAINER_NAME":"k8s_not-black_not-black-79d574774-2rxrj_kube-system_a093cbca-fb5a-11e7-a6b6-06263dd4a414_6","CONTAINER_TAG":"%s@sha256:6ceb111a36020dc2124c0d7e3746088c20c7e3806a1075dd9e5fe1c42f744fff","HOSTNAME":"ip-127-0-0-1.north-1.compute.internal","MACHINE_ID":"8d1225f40ee64cc7bcce2f549a41657c","MESSAGE":"INFO  [2019-04-09 09:18:49,172] com.ft.ingester.ingestion.Ingester: transaction_id=republish_tid_PFuz36ssPr outcome=Success cmspublicationevent=http://methode-image-model-mapper.svc.ft.com/image/model/341826dc-55b1-11e9-91f9-b6515a54c5b1 message=Publish[kafka-handlers-1]"POD_NAME":"cluster-autoscaler-79d574774-2rxrj","SERVICE_NAME":"whatever-api","SYSTEMD_UNIT":"docker.service","_SOURCE_REALTIME_TIMESTAMP":"1516376285932645","_SYSTEMD_INVOCATION_ID":"e3b2703c430f45e8a7075dbcf6b3a588","environment":"upp-prod-publish-eu","platform":"up-coco"}`,
	},
}

func unprocessedMessages() []map[string]interface{} {
	rawMessages := []string{
		`{ "__CURSOR" : "s=173daef302184e64b1c9324703ad9052;i=3f3b796;b=fffc116fc4e2499b8a7bbca148f5e589;m=5356041931;t=586178fd57792;x=734ed5bd3a101434", "__REALTIME_TIMESTAMP" : "1554810639054738", "__MONOTONIC_TIMESTAMP" : "357925394737", "_BOOT_ID" : "fffc116fc4e2499b8a7bbca148f5e589", "PRIORITY" : "6", "CONTAINER_ID_FULL" : "6ed8a10c174841d76232bd4128ebefb0cab8c517090c66f09c83591e748f5fb1", "CONTAINER_NAME" : "k8s_delivery-varnish_delivery-varnish-74d6bf5d-lrxj4_default_1793dbd7-5786-11e9-8de2-067a2aa9d532_3", "CONTAINER_TAG" : "sha256:0af93f5894e91bc1630a074ff7a88e6656d255b9e15534f3e9405d4f544f9f59", "SYSLOG_IDENTIFIER" : "sha256:0af93f5894e91bc1630a074ff7a88e6656d255b9e15534f3e9405d4f544f9f59", "CONTAINER_ID" : "6ed8a10c1748", "_TRANSPORT" : "journal", "_PID" : "1927", "_UID" : "0", "_GID" : "0", "_COMM" : "dockerd", "_EXE" : "/run/torcx/unpack/docker/bin/dockerd", "_CMDLINE" : "/run/torcx/bin/dockerd --host=fd:// --containerd=/var/run/docker/libcontainerd/docker-containerd.sock --selinux-enabled=true --log-driver=journald --host 0.0.0.0:2375 --log-opt tag={{.ImageName}}", "_CAP_EFFECTIVE" : "3fffffffff", "_SELINUX_CONTEXT" : "system_u:system_r:kernel_t:s0", "_SYSTEMD_CGROUP" : "/system.slice/docker.service", "_SYSTEMD_UNIT" : "docker.service", "_SYSTEMD_SLICE" : "system.slice", "_SYSTEMD_INVOCATION_ID" : "9e1c191cd75a4e239e27c4fb3c1b88c5", "_MACHINE_ID" : "foobar", "_HOSTNAME" : "ip-127-0-0-1.north-1.compute.internal", "MESSAGE" : "127.0.0.1, 127.0.0.1, 127.0.0.1, 127.0.0.1 api-gateway-pre-prod-20170112 09/Apr/2019:11:50:39 /content/4f2f97ea-b8ec-11e4-b8e6-00144feab7de 200 251 \"PAC-draft-content-api/0.0.13\" transaction_id=06602191-F8C5-47DA-BDCE-73DAE0C7704D hit", "_SOURCE_REALTIME_TIMESTAMP" : "1554810639054727" }`,
		`{ "__CURSOR" : "s=173daef302184e64b1c9324703ad9052;i=3f3b797;b=fffc116fc4e2499b8a7bbca148f5e589;m=53560419b0;t=586178fd57811;x=2f3a9d59be927de3", "__REALTIME_TIMESTAMP" : "1554810639054865", "__MONOTONIC_TIMESTAMP" : "357925394864", "_BOOT_ID" : "fffc116fc4e2499b8a7bbca148f5e589", "PRIORITY" : "6", "CONTAINER_ID_FULL" : "6ed8a10c174841d76232bd4128ebefb0cab8c517090c66f09c83591e748f5fb1", "CONTAINER_NAME" : "k8s_delivery-varnish_delivery-varnish-74d6bf5d-lrxj4_default_1793dbd7-5786-11e9-8de2-067a2aa9d532_3", "CONTAINER_TAG" : "sha256:0af93f5894e91bc1630a074ff7a88e6656d255b9e15534f3e9405d4f544f9f59", "SYSLOG_IDENTIFIER" : "sha256:0af93f5894e91bc1630a074ff7a88e6656d255b9e15534f3e9405d4f544f9f59", "CONTAINER_ID" : "6ed8a10c1748", "_TRANSPORT" : "journal", "_PID" : "1927", "_UID" : "0", "_GID" : "0", "_COMM" : "dockerd", "_EXE" : "/run/torcx/unpack/docker/bin/dockerd", "_CMDLINE" : "/run/torcx/bin/dockerd --host=fd:// --containerd=/var/run/docker/libcontainerd/docker-containerd.sock --selinux-enabled=true --log-driver=journald --host 0.0.0.0:2375 --log-opt tag={{.ImageName}}", "_CAP_EFFECTIVE" : "3fffffffff", "_SELINUX_CONTEXT" : "system_u:system_r:kernel_t:s0", "_SYSTEMD_CGROUP" : "/system.slice/docker.service", "_SYSTEMD_UNIT" : "docker.service", "_SYSTEMD_SLICE" : "system.slice", "_SYSTEMD_INVOCATION_ID" : "9e1c191cd75a4e239e27c4fb3c1b88c5", "_MACHINE_ID" : "ec25f9924866ce04917b0618498f51e8", "_HOSTNAME" : "ip-127-0-0-1.north-1.compute.internal", "MESSAGE" : "127.0.0.1, 127.0.0.1 load-replicator-20180529 09/Apr/2019:11:50:39 /enrichedcontent/3d0eab7e-de90-11e7-3eb2-9ede1f80350f 200 9406 \"load-replicator\" transaction_id=- miss", "_SOURCE_REALTIME_TIMESTAMP" : "1554810639054742" },`,
		`{ "__CURSOR" : "s=173daef302184e64b1c9324703ad9052;i=3f3b798;b=fffc116fc4e2499b8a7bbca148f5e589;m=5356046fac;t=586178fd5ce0c;x=8bf3164672134272", "__REALTIME_TIMESTAMP" : "1554810639076876", "__MONOTONIC_TIMESTAMP" : "357925416876", "_BOOT_ID" : "fffc116fc4e2499b8a7bbca148f5e589", "PRIORITY" : "6", "_TRANSPORT" : "journal", "_PID" : "1927", "_UID" : "0", "_GID" : "0", "_COMM" : "dockerd", "_EXE" : "/run/torcx/unpack/docker/bin/dockerd", "_CMDLINE" : "/run/torcx/bin/dockerd --host=fd:// --containerd=/var/run/docker/libcontainerd/docker-containerd.sock --selinux-enabled=true --log-driver=journald --host 0.0.0.0:2375 --log-opt tag={{.ImageName}}", "_CAP_EFFECTIVE" : "3fffffffff", "_SELINUX_CONTEXT" : "system_u:system_r:kernel_t:s0", "_SYSTEMD_CGROUP" : "/system.slice/docker.service", "_SYSTEMD_UNIT" : "docker.service", "_SYSTEMD_SLICE" : "system.slice", "_SYSTEMD_INVOCATION_ID" : "9e1c191cd75a4e239e27c4fb3c1b88c5", "_MACHINE_ID" : "ec25f9924866ce04917b0618498f51e8", "_HOSTNAME" : "ip-127-0-0-1.north-1.compute.internal", "CONTAINER_ID" : "5cee67a328f4", "CONTAINER_ID_FULL" : "5cee67a328f4f409b97bc1e15198b43bb73e7ee518a516b354bf9ff41c8a1d17", "CONTAINER_NAME" : "k8s_content-public-read_content-public-read-764898846b-vbq7d_default_16157552-5786-11e9-8de2-067a2aa9d532_0", "CONTAINER_TAG" : "content-public-read@sha256:c78c62caf349bcea51157357fbc612816e8a240fa893f5c9ede0bdf2b9fed1ae", "SYSLOG_IDENTIFIER" : "content-public-read@sha256:c78c62caf349bcea51157357fbc612816e8a240fa893f5c9ede0bdf2b9fed1ae", "MESSAGE" : "INFO  [2019-04-09 11:50:39,076] com.ft.contentpublicread.service.rest.RestRemoteContentService: transaction_id=tid_3acaxekfyd Calling Content endpoint: http://service:8080/content/81c56de8-de90-11e7-a0d4-0944c5f49e46|[dw-217847 - GET /content/81c56de8-de90-11e7-a0d4-0944c5f49e46]", "_SOURCE_REALTIME_TIMESTAMP" : "1554810639076858" }`,
		`{ "__CURSOR" : "s=173daef302184e64b1c9324703ad9052;i=3f3b799;b=fffc116fc4e2499b8a7bbca148f5e589;m=5356049121;t=586178fd5ef81;x=28ec096e24be09f0", "__REALTIME_TIMESTAMP" : "1554810639085441", "__MONOTONIC_TIMESTAMP" : "357925425441", "_BOOT_ID" : "fffc116fc4e2499b8a7bbca148f5e589", "PRIORITY" : "6", "_TRANSPORT" : "journal", "_PID" : "1927", "_UID" : "0", "_GID" : "0", "_COMM" : "dockerd", "_EXE" : "/run/torcx/unpack/docker/bin/dockerd", "_CMDLINE" : "/run/torcx/bin/dockerd --host=fd:// --containerd=/var/run/docker/libcontainerd/docker-containerd.sock --selinux-enabled=true --log-driver=journald --host 0.0.0.0:2375 --log-opt tag={{.ImageName}}", "_CAP_EFFECTIVE" : "3fffffffff", "_SELINUX_CONTEXT" : "system_u:system_r:kernel_t:s0", "_SYSTEMD_CGROUP" : "/system.slice/docker.service", "_SYSTEMD_UNIT" : "docker.service", "_SYSTEMD_SLICE" : "system.slice", "_SYSTEMD_INVOCATION_ID" : "9e1c191cd75a4e239e27c4fb3c1b88c5", "_MACHINE_ID" : "ec25f9924866ce04917b0618498f51e8", "_HOSTNAME" : "ip-127-0-0-1.north-1.compute.internal", "CONTAINER_ID" : "5cee67a328f4", "CONTAINER_ID_FULL" : "5cee67a328f4f409b97bc1e15198b43bb73e7ee518a516b354bf9ff41c8a1d17", "CONTAINER_NAME" : "k8s_content-public-read_content-public-read-764898846b-vbq7d_default_16157552-5786-11e9-8de2-067a2aa9d532_0", "CONTAINER_TAG" : "content-public-read@sha256:c78c62caf349bcea51157357fbc612816e8a240fa893f5c9ede0bdf2b9fed1ae", "SYSLOG_IDENTIFIER" : "content-public-read@sha256:c78c62caf349bcea51157357fbc612816e8a240fa893f5c9ede0bdf2b9fed1ae", "MESSAGE" : "10.2.6.76 - - [09/Apr/2019:11:50:39 +0000] \"GET /content/81c56de8-de90-11e7-a0d4-0944c5f49e46 HTTP/1.1\" 200 1159 \"-\" \"Resilient Client (v=0.3-SNAPSHOT, sn=contentApi, transaction_id=tid_3acaxekfyd)\" 9", "_SOURCE_REALTIME_TIMESTAMP" : "1554810639085425" }`,
		`{ "__CURSOR" : "s=173daef302184e64b1c9324703ad9052;i=3f3b79a;b=fffc116fc4e2499b8a7bbca148f5e589;m=535604a5c3;t=586178fd60423;x=dc01f7d6827d0a6c", "__REALTIME_TIMESTAMP" : "1554810639090723", "__MONOTONIC_TIMESTAMP" : "357925430723", "_BOOT_ID" : "fffc116fc4e2499b8a7bbca148f5e589", "PRIORITY" : "6", "_TRANSPORT" : "journal", "_PID" : "1927", "_UID" : "0", "_GID" : "0", "_COMM" : "dockerd", "_EXE" : "/run/torcx/unpack/docker/bin/dockerd", "_CMDLINE" : "/run/torcx/bin/dockerd --host=fd:// --containerd=/var/run/docker/libcontainerd/docker-containerd.sock --selinux-enabled=true --log-driver=journald --host 0.0.0.0:2375 --log-opt tag={{.ImageName}}", "_CAP_EFFECTIVE" : "3fffffffff", "_SELINUX_CONTEXT" : "system_u:system_r:kernel_t:s0", "_SYSTEMD_CGROUP" : "/system.slice/docker.service", "_SYSTEMD_UNIT" : "docker.service", "_SYSTEMD_SLICE" : "system.slice", "_SYSTEMD_INVOCATION_ID" : "9e1c191cd75a4e239e27c4fb3c1b88c5", "_MACHINE_ID" : "ec25f9924866ce04917b0618498f51e8", "_HOSTNAME" : "ip-127-0-0-1.north-1.compute.internal", "CONTAINER_ID" : "5cee67a328f4", "CONTAINER_ID_FULL" : "5cee67a328f4f409b97bc1e15198b43bb73e7ee518a516b354bf9ff41c8a1d17", "CONTAINER_NAME" : "k8s_content-public-read_content-public-read-764898846b-vbq7d_default_16157552-5786-11e9-8de2-067a2aa9d532_0", "CONTAINER_TAG" : "content-public-read@sha256:c78c62caf349bcea51157357fbc612816e8a240fa893f5c9ede0bdf2b9fed1ae", "SYSLOG_IDENTIFIER" : "content-public-read@sha256:c78c62caf349bcea51157357fbc612816e8a240fa893f5c9ede0bdf2b9fed1ae", "MESSAGE" : "INFO  [2019-04-09 11:50:39,090] com.ft.contentpublicread.service.rest.RestRemoteContentService: transaction_id=tid_sw9qvxvtey Calling Content endpoint: http://service:8080/content/2fd17e43-2fe9-3172-8168-5cedc630a961|[dw-217074 - GET /content/2fd17e43-2fe9-3172-8168-5cedc630a961]", "_SOURCE_REALTIME_TIMESTAMP" : "1554810639090710" }`,
		`{ "__CURSOR" : "s=173daef302184e64b1c9324703ad9052;i=3f3b79b;b=fffc116fc4e2499b8a7bbca148f5e589;m=535604db27;t=586178fd63987;x=2e8d17a2d0e387c3", "__REALTIME_TIMESTAMP" : "1554810639104391", "__MONOTONIC_TIMESTAMP" : "357925444391", "_BOOT_ID" : "fffc116fc4e2499b8a7bbca148f5e589", "PRIORITY" : "6", "CONTAINER_ID_FULL" : "6ed8a10c174841d76232bd4128ebefb0cab8c517090c66f09c83591e748f5fb1", "CONTAINER_NAME" : "k8s_delivery-varnish_delivery-varnish-74d6bf5d-lrxj4_default_1793dbd7-5786-11e9-8de2-067a2aa9d532_3", "CONTAINER_TAG" : "sha256:0af93f5894e91bc1630a074ff7a88e6656d255b9e15534f3e9405d4f544f9f59", "SYSLOG_IDENTIFIER" : "sha256:0af93f5894e91bc1630a074ff7a88e6656d255b9e15534f3e9405d4f544f9f59", "CONTAINER_ID" : "6ed8a10c1748", "_TRANSPORT" : "journal", "_PID" : "1927", "_UID" : "0", "_GID" : "0", "_COMM" : "dockerd", "_EXE" : "/run/torcx/unpack/docker/bin/dockerd", "_CMDLINE" : "/run/torcx/bin/dockerd --host=fd:// --containerd=/var/run/docker/libcontainerd/docker-containerd.sock --selinux-enabled=true --log-driver=journald --host 0.0.0.0:2375 --log-opt tag={{.ImageName}}", "_CAP_EFFECTIVE" : "3fffffffff", "_SELINUX_CONTEXT" : "system_u:system_r:kernel_t:s0", "_SYSTEMD_CGROUP" : "/system.slice/docker.service", "_SYSTEMD_UNIT" : "docker.service", "_SYSTEMD_SLICE" : "system.slice", "_SYSTEMD_INVOCATION_ID" : "9e1c191cd75a4e239e27c4fb3c1b88c5", "_MACHINE_ID" : "ec25f9924866ce04917b0618498f51e8", "_HOSTNAME" : "ip-127-0-0-1.north-1.compute.internal", "MESSAGE" : "127.0.0.1, 127.0.0.1 load-replicator-20180529 09/Apr/2019:11:50:39 /lists/8d5b0e30-55d6-11e7-80b6-9bfa4c1f83d2 200 195 \"load-replicator\" transaction_id=- hit", "_SOURCE_REALTIME_TIMESTAMP" : "1554810639104376" }`,
		`{ "__CURSOR" : "s=173daef302184e64b1c9324703ad9052;i=3f3b79c;b=fffc116fc4e2499b8a7bbca148f5e589;m=535604e2d3;t=586178fd64133;x=b8f197f12e6163b6", "__REALTIME_TIMESTAMP" : "1554810639106355", "__MONOTONIC_TIMESTAMP" : "357925446355", "_BOOT_ID" : "fffc116fc4e2499b8a7bbca148f5e589", "PRIORITY" : "6", "_TRANSPORT" : "journal", "_PID" : "1927", "_UID" : "0", "_GID" : "0", "_COMM" : "dockerd", "_EXE" : "/run/torcx/unpack/docker/bin/dockerd", "_CMDLINE" : "/run/torcx/bin/dockerd --host=fd:// --containerd=/var/run/docker/libcontainerd/docker-containerd.sock --selinux-enabled=true --log-driver=journald --host 0.0.0.0:2375 --log-opt tag={{.ImageName}}", "_CAP_EFFECTIVE" : "3fffffffff", "_SELINUX_CONTEXT" : "system_u:system_r:kernel_t:s0", "_SYSTEMD_CGROUP" : "/system.slice/docker.service", "_SYSTEMD_UNIT" : "docker.service", "_SYSTEMD_SLICE" : "system.slice", "_SYSTEMD_INVOCATION_ID" : "9e1c191cd75a4e239e27c4fb3c1b88c5", "_MACHINE_ID" : "ec25f9924866ce04917b0618498f51e8", "_HOSTNAME" : "ip-127-0-0-1.north-1.compute.internal", "CONTAINER_ID" : "5cee67a328f4", "CONTAINER_ID_FULL" : "5cee67a328f4f409b97bc1e15198b43bb73e7ee518a516b354bf9ff41c8a1d17", "CONTAINER_NAME" : "k8s_content-public-read_content-public-read-764898846b-vbq7d_default_16157552-5786-11e9-8de2-067a2aa9d532_0", "CONTAINER_TAG" : "content-public-read@sha256:c78c62caf349bcea51157357fbc612816e8a240fa893f5c9ede0bdf2b9fed1ae", "SYSLOG_IDENTIFIER" : "content-public-read@sha256:c78c62caf349bcea51157357fbc612816e8a240fa893f5c9ede0bdf2b9fed1ae", "MESSAGE" : "127.0.0.1 - - [09/Apr/2019:11:50:39 +0000] \"GET /content/2fd17e43-2fe9-3172-8168-5cedc630a961 HTTP/1.1\" 200 3187 \"-\" \"load-replicator\" 17", "_SOURCE_REALTIME_TIMESTAMP" : "1554810639106343" }`,
		`{ "__CURSOR" : "s=173daef302184e64b1c9324703ad9052;i=3f3b79d;b=fffc116fc4e2499b8a7bbca148f5e589;m=535604e446;t=586178fd642a6;x=fe81daf092a52395", "__REALTIME_TIMESTAMP" : "1554810639106726", "__MONOTONIC_TIMESTAMP" : "357925446726", "_BOOT_ID" : "fffc116fc4e2499b8a7bbca148f5e589", "_TRANSPORT" : "journal", "_PID" : "1927", "_UID" : "0", "_GID" : "0", "_COMM" : "dockerd", "_EXE" : "/run/torcx/unpack/docker/bin/dockerd", "_CMDLINE" : "/run/torcx/bin/dockerd --host=fd:// --containerd=/var/run/docker/libcontainerd/docker-containerd.sock --selinux-enabled=true --log-driver=journald --host 0.0.0.0:2375 --log-opt tag={{.ImageName}}", "_CAP_EFFECTIVE" : "3fffffffff", "_SELINUX_CONTEXT" : "system_u:system_r:kernel_t:s0", "_SYSTEMD_CGROUP" : "/system.slice/docker.service", "_SYSTEMD_UNIT" : "docker.service", "_SYSTEMD_SLICE" : "system.slice", "_SYSTEMD_INVOCATION_ID" : "9e1c191cd75a4e239e27c4fb3c1b88c5", "_MACHINE_ID" : "ec25f9924866ce04917b0618498f51e8", "_HOSTNAME" : "ip-127-0-0-1.north-1.compute.internal", "PRIORITY" : "3", "CONTAINER_TAG" : "coco/relations-api@sha256:17c9e0533e744b8d3611705ac9654969f6cd1b4805f42dd6ebea6c65d124784e", "SYSLOG_IDENTIFIER" : "coco/relations-api@sha256:17c9e0533e744b8d3611705ac9654969f6cd1b4805f42dd6ebea6c65d124784e", "CONTAINER_ID" : "f665773e38e3", "CONTAINER_ID_FULL" : "f665773e38e3311516d57ca5890aeee3484c2f1c0797b5b08769ef4e31bcb761", "CONTAINER_NAME" : "k8s_relations-api_relations-api-f7f764695-h9d6f_default_1d72e448-5786-11e9-8de2-067a2aa9d532_0", "MESSAGE" : "{\"@time\":\"2019-04-09T11:50:39.106569704Z\",\"host\":\"127.0.0.1\",\"level\":\"info\",\"method\":\"GET\",\"msg\":\"\",\"protocol\":\"HTTP/1.1\",\"referer\":\"\",\"responsetime\":5,\"service_name\":\"relations-api-neo4j\",\"size\":91,\"status\":404,\"transaction_id\":\"tid_dtygyxllr8\",\"uri\":\"/content/81c56de8-de90-11e7-3eb2-9ede1f80350f/relations\",\"userAgent\":\"Resilient Client (v=0.3-SNAPSHOT, sn=relationsApi, transaction_id=tid_dtygyxllr8)\",\"username\":\"-\"}", "_SOURCE_REALTIME_TIMESTAMP" : "1554810639106706" }`,
		`{ "__CURSOR" : "s=173daef302184e64b1c9324703ad9052;i=3f3b79e;b=fffc116fc4e2499b8a7bbca148f5e589;m=5356052aa0;t=586178fd68900;x=70b81699985c0bb", "__REALTIME_TIMESTAMP" : "1554810639124736", " __MONOTONIC_TIMESTAMP" : "357925464736", "_BOOT_ID" : "fffc116fc4e2499b8a7bbca148f5e589", "PRIORITY" : "6", "CONTAINER_ID_FULL" : "6ed8a10c174841d76232bd4128ebefb0cab8c517090c66f09c83591e748f5fb1", "CONTAINER_NAME" : "k8s_delivery-varnish_delivery-varnish-74d6bf5d-lrxj4_default_1793dbd7-5786-11e9-8de2-067a2aa9d532_3", "CONTAINER_TAG" : "sha256:0af93f5894e91bc1630a074ff7a88e6656d255b9e15534f3e9405d4f544f9f59", "SYSLOG_IDENTIFIER" : "sha256:0af93f5894e91bc1630a074ff7a88e6656d255b9e15534f3e9405d4f544f9f59", "CONTAINER_ID" : "6ed8a10c1748", "_TRANSPORT" : "journal", "_PID" : "1927", "_UID" : "0", "_GID" : "0", "_COMM" : "dockerd", "_EXE" : "/run/torcx/unpack/docker/bin/dockerd", "_CMDLINE" : "/run/torcx/bin/dockerd --host=fd:// --containerd=/var/run/docker/libcontainerd/docker-containerd.sock --selinux-enabled=true --log-driver=journald --host 0.0.0.0:2375 --log-opt tag={{.ImageName}}", "_CAP_EFFECTIVE" : "3fffffffff", "_SELINUX_CONTEXT" : "system_u:system_r:kernel_t:s0", "_SYSTEMD_CGROUP" : "/system.slice/docker.service", "_SYSTEMD_UNIT" : "docker.service", "_SYSTEMD_SLICE" : "system.slice", "_SYSTEMD_INVOCATION_ID" : "9e1c191cd75a4e239e27c4fb3c1b88c5", "_MACHINE_ID" : "ec25f9924866ce04917b0618498f51e8", "_HOSTNAME" : "ip-127-0-0-1.north-1.compute.internal", "MESSAGE" : "127.0.0.1, 127.0.0.1 load-replicator-20180529 09/Apr/2019:11:50:39 /enrichedcontent/81c56de8-de90-11e7-3eb2-9ede1f80350f 200 26167 \"load-replicator\" transaction_id=- miss", "_SOURCE_REALTIME_TIMESTAMP" : "1554810639124722" }`,
		`{ "__CURSOR" : "s=173daef302184e64b1c9324703ad9052;i=3f3b79f;b=fffc116fc4e2499b8a7bbca148f5e589;m=535605523d;t=586178fd6b09d;x=501fb74b6a3494bd", "__REALTIME_TIMESTAMP" : "1554810639134877", "__MONOTONIC_TIMESTAMP" : "357925474877", "_BOOT_ID" : "fffc116fc4e2499b8a7bbca148f5e589", "PRIORITY" : "6", "CONTAINER_ID_FULL" : "6ed8a10c174841d76232bd4128ebefb0cab8c517090c66f09c83591e748f5fb1", "CONTAINER_NAME" : "k8s_delivery-varnish_delivery-varnish-74d6bf5d-lrxj4_default_1793dbd7-5786-11e9-8de2-067a2aa9d532_3", "CONTAINER_TAG" : "sha256:0af93f5894e91bc1630a074ff7a88e6656d255b9e15534f3e9405d4f544f9f59", "SYSLOG_IDENTIFIER" : "sha256:0af93f5894e91bc1630a074ff7a88e6656d255b9e15534f3e9405d4f544f9f59", "CONTAINER_ID" : "6ed8a10c1748", "_TRANSPORT" : "journal", "_PID" : "1927", "_UID" : "0", "_GID" : "0", "_COMM" : "dockerd", "_EXE" : "/run/torcx/unpack/docker/bin/dockerd", "_CMDLINE" : "/run/torcx/bin/dockerd --host=fd:// --containerd=/var/run/docker/libcontainerd/docker-containerd.sock --selinux-enabled=true --log-driver=journald --host 0.0.0.0:2375 --log-opt tag={{.ImageName}}", "_CAP_EFFECTIVE" : "3fffffffff", "_SELINUX_CONTEXT" : "system_u:system_r:kernel_t:s0", "_SYSTEMD_CGROUP" : "/system.slice/docker.service", "_SYSTEMD_UNIT" : "docker.service", "_SYSTEMD_SLICE" : "system.slice", "_SYSTEMD_INVOCATION_ID" : "9e1c191cd75a4e239e27c4fb3c1b88c5", "_MACHINE_ID" : "ec25f9924866ce04917b0618498f51e8", "_HOSTNAME" : "ip-127-0-0-1.north-1.compute.internal", "MESSAGE" : "127.0.0.1, 127.0.0.1 load-replicator-20180529 09/Apr/2019:11:50:39 /enrichedcontent/35da25c4-dc0e-11e7-a039-c64b1c09b482 200 18017 \"load-replicator\" transaction_id=- miss", "_SOURCE_REALTIME_TIMESTAMP" : "1554810639134854" }`,
	}
	jsonMessages := make([]map[string]interface{}, len(rawMessages))
	for i, message := range rawMessages {
		m := make(map[string]interface{})
		_ = json.NewDecoder(strings.NewReader(message)).Decode(&m)
		jsonMessages[i] = m
	}
	return jsonMessages
}

var containerNames = []string{
	"k8s_notifications-rw_notifications-rw-68fbb7d959-4cmd8_default_1a0d2b1b-5786-11e9-8de2-067a2aa9d532_10",
	"k8s_content-public-read_content-public-read-764898846b-vbq7d_default_16157552-5786-11e9-8de2-067a2aa9d532_0",
	"k8s_methode-article-mapper_methode-article-mapper-6dbfc95dc-7gtzw_default_5afc51f5-57a4-11e9-814a-0a5b4b43b40a_0",
	"k8s_methode-image-model-mapper_methode-image-model-mapper-8467d9898b-82rhb_default_5bd34bb4-57a4-11e9-814a-0a5b4b43b40a_0",
	"k8s_kiam_kiam-agent-nxtgv_kube-system_ae8d3110-577c-11e9-8de2-067a2aa9d532_4",
}

var processedMessages = []string{
	`127.0.0.1, 127.0.0.1, 127.0.0.1, 127.0.0.1 api-gateway-pre-prod-20170112 09/Apr/2019:11:50:39 /content/4f2f97ea-b8ec-11e4-b8e6-00144feab7de 200 251 "PAC-draft-content-api/0.0.13" transaction_id=06602191-F8C5-47DA-BDCE-73DAE0C7704D hit`,
	`127.0.0.1, 127.0.0.1 load-replicator-20180529 09/Apr/2019:11:50:39 /enrichedcontent/3d0eab7e-de90-11e7-3eb2-9ede1f80350f 200 9406 "load-replicator" transaction_id=- miss`,
	`INFO  [2019-04-09 11:50:39,076] com.ft.contentpublicread.service.rest.RestRemoteContentService: transaction_id=tid_3acaxekfyd Calling Content endpoint: http://service:8080/content/81c56de8-de90-11e7-a0d4-0944c5f49e46|[dw-217847 - GET /content/81c56de8-de90-11e7-a0d4-0944c5f49e46]`,
	`127.0.0.1 - - [09/Apr/2019:11:50:39 +0000] "GET /content/81c56de8-de90-11e7-a0d4-0944c5f49e46 HTTP/1.1" 200 1159 "-" "Resilient Client (v=0.3-SNAPSHOT, sn=contentApi, transaction_id=tid_3acaxekfyd)" 9`,
	`INFO  [2019-04-09 11:50:39,090] com.ft.contentpublicread.service.rest.RestRemoteContentService: transaction_id=tid_sw9qvxvtey Calling Content endpoint: http://service:8080/content/2fd17e43-2fe9-3172-8168-5cedc630a961|[dw-217074 - GET /content/2fd17e43-2fe9-3172-8168-5cedc630a961]`,
	`127.0.0.1, 127.0.0.1 load-replicator-20180529 09/Apr/2019:11:50:39 /lists/8d5b0e30-55d6-11e7-80b6-9bfa4c1f83d2 200 195 "load-replicator" transaction_id=- hit`,
	`127.0.0.1 - - [09/Apr/2019:11:50:39 +0000] "GET /content/2fd17e43-2fe9-3172-8168-5cedc630a961 HTTP/1.1" 200 3187 "-" "load-replicator" 17`,
	`{"@time":"2019-04-09T11:50:39.106569704Z","host":"10.2.6.76","level":"info","method":"GET","msg":"","protocol":"HTTP/1.1","referer":"","responsetime":5,"service_name":"relations-api-neo4j","size":91,"status":404,"transaction_id":"tid_dtygyxllr8","uri":"/content/81c56de8-de90-11e7-3eb2-9ede1f80350f/relations","userAgent":"Resilient Client (v=0.3-SNAPSHOT, sn=relationsApi, transaction_id=tid_dtygyxllr8)","username":"-"}`,
	`127.0.0.1, 127.0.0.1 load-replicator-20180529 09/Apr/2019:11:50:39 /enrichedcontent/81c56de8-de90-11e7-3eb2-9ede1f80350f 200 26167 "load-replicator" transaction_id=- miss`,
	`127.0.0.1, 127.0.0.1 load-replicator-20180529 09/Apr/2019:11:50:39 /enrichedcontent/35da25c4-dc0e-11e7-a039-c64b1c09b482 200 18017 "load-replicator" transaction_id=- miss`,
}

var messagesWithApiKeys = []string{
	`10.2.26.0 ops-17-01-2018 30/Jan/2018:08:35:04 /content/notifications-push?apiKey=vhs2aazf3gyywm3wk2sv44wb&type=ALL 200 -2147483648 "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.132 Safari/537.36" transaction_id=- miss`,
	`10.2.26.0 ops-17-01-2018 30/Jan/2018:08:35:04 /content/notifications-push?apiKey=vhs2aazf3gyywm3wk2sv44wb&type=ALL /content/notifications-push?api_key=wm3wk2sv44wbvhs2aazf3gyy`,
	`10.2.26.0 ops-17-01-2018 30/Jan/2018:08:35:04 /content/notifications-push?type=ALL 200 -2147483648 "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.132 Safari/537.36" transaction_id=- miss`,
}
