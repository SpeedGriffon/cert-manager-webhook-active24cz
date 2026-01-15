package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

type Client struct {
	httpClient *http.Client
	Config     *Config
}

type Config struct {
	ApiKey    string
	ApiSecret string
	DnsZone   string
	ServiceId string
}

type DnsRecord struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Ttl     int    `json:"ttl"`
}

type DnsRecordWithId struct {
	Id int `json:"id"`
	DnsRecord
}

type DnsRecords struct {
	Data []DnsRecordWithId `json:"data"`
}

type DnsFilters struct {
	Name    string   `json:"name"`
	Type    []string `json:"type"`
	Content string   `json:"content"`
	Ttl     int      `json:"ttl"`
}

type DnsGetParams struct {
	Filters DnsFilters `json:"filters"`
}

// A method expression matching both CreateRecord and DeleteRecord.
type Request func(*Client, *DnsRecord) error

// Construct GET request parameters.
func (r *DnsRecord) GetParams() *DnsGetParams {
	return &DnsGetParams{
		Filters: DnsFilters{
			Name:    r.Name,
			Type:    []string{r.Type},
			Content: r.Content,
			Ttl:     r.Ttl,
		},
	}
}

// Create a new Client instance.
func NewClient(config *Config) *Client {
	httpClient := &http.Client{
		Timeout: 60 * time.Second,
	}
	return &Client{
		httpClient: httpClient,
		Config:     config,
	}
}

// Get base API URL.
func (c *Client) BaseUrl() string {
	return "https://rest.active24.cz"
}

// Get full API URL.
func (c *Client) ApiUrl(id *int) string {
	s := ""
	if id != nil {
		s = fmt.Sprintf("/%d", *id)
	}
	return fmt.Sprintf("%s/v2/service/%s/dns/record%s", c.BaseUrl(), c.Config.ServiceId, s)
}

// Get a signature hash.
func signatureHash(secret string, method string, path string, time int64) string {
	h := hmac.New(sha1.New, []byte(secret))

	_, _ = fmt.Fprintf(h, "%s %s %d", method, path, time)

	return fmt.Sprintf("%x", h.Sum(nil))
}

// Create a new HTTP request.
func (c *Client) newRequest(method string, url string, data []byte) (*http.Request, error) {
	req, err := http.NewRequest(method, url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	timestamp := time.Now().UTC()

	signature := signatureHash(c.Config.ApiSecret, method, req.URL.Path, timestamp.Unix())

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Date", timestamp.Format(time.RFC3339))

	req.SetBasicAuth(c.Config.ApiKey, signature)
	return req, nil
}

// Make an API request.
func (c *Client) Request(method string, id *int, data []byte) ([]byte, error) {
	req, err := c.newRequest(method, c.ApiUrl(id), data)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("api request failed: %s", body)
	}

	return body, nil
}

// Find matching DNS records.
func (c *Client) FindRecords(record *DnsRecord) ([]DnsRecordWithId, error) {
	data, err := json.Marshal(record.GetParams())
	if err != nil {
		return nil, err
	}

	resp, err := c.Request("GET", nil, data)
	if err != nil {
		return nil, err
	}

	records := DnsRecords{}
	err = json.Unmarshal(resp, &records)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal api response %s: %w", resp, err)
	}

	return records.Data, nil
}

// Get a DNS record's ID.
func (c *Client) GetRecordId(record *DnsRecord) (*int, error) {
	foundRecords, err := c.FindRecords(record)
	if err != nil {
		return nil, err
	}

	if len(foundRecords) == 0 {
		return nil, nil
	}

	if len(foundRecords) == 1 {
		f := &foundRecords[0]
		f.Name = strings.TrimSuffix(f.Name, c.Config.DnsZone)
		if *record == f.DnsRecord {
			return &f.Id, nil
		}
	}

	return nil, fmt.Errorf("expected record %v, found %v", record, foundRecords)
}

// Create a DNS record.
func (c *Client) CreateRecord(record *DnsRecord) error {
	id, err := c.GetRecordId(record)
	if err != nil {
		return err
	}
	if id != nil {
		klog.Infof("record %v already exists", record)
		return nil
	}

	data, err := json.Marshal(record)
	if err != nil {
		return err
	}

	klog.Infof("creating record %v", record)
	_, err = c.Request("POST", nil, data)
	return err
}

// Delete a DNS record.
func (c *Client) DeleteRecord(record *DnsRecord) error {
	id, err := c.GetRecordId(record)
	if err != nil {
		return err
	}
	if id == nil {
		klog.Infof("record %v already gone", record)
		return nil
	}

	klog.Infof("deleting record %v", record)
	_, err = c.Request("DELETE", id, nil)
	return err
}
