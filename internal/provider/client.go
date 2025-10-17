package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// StalwartClient is the client for interacting with the Stalwart API
type StalwartClient struct {
	ServerHostname string
	ApiKey         string
	httpClient     *http.Client
}

// getHTTPClient returns the HTTP client, creating it if needed
func (c *StalwartClient) getHTTPClient() *http.Client {
	if c.httpClient == nil {
		c.httpClient = &http.Client{}
	}
	return c.httpClient
}

// doRequest performs an HTTP request with common headers and automatic retry on rate limiting
func (c *StalwartClient) doRequest(method, path string, body interface{}) (*http.Response, error) {
	url := fmt.Sprintf("https://%s%s", c.ServerHostname, path)

	var jsonData []byte
	if body != nil {
		var err error
		jsonData, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("error marshaling request body: %w", err)
		}
	}

	// Retry up to 5 times with exponential backoff
	maxRetries := 5
	for attempt := 0; attempt <= maxRetries; attempt++ {
		var reqBody io.Reader
		if jsonData != nil {
			reqBody = bytes.NewBuffer(jsonData)
		}

		req, err := http.NewRequest(method, url, reqBody)
		if err != nil {
			return nil, fmt.Errorf("error creating request: %w", err)
		}

		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.ApiKey))
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.getHTTPClient().Do(req)
		if err != nil {
			return nil, fmt.Errorf("error executing request: %w", err)
		}

		// If not rate limited, return immediately
		if resp.StatusCode != http.StatusTooManyRequests {
			return resp, nil
		}

		// If we've exhausted retries, return the rate limit error
		if attempt == maxRetries {
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("rate limited after %d retries: %s", maxRetries, string(body))
		}

		// Close the response body before retrying
		resp.Body.Close()

		// Wait with exponential backoff: 1s, 2s, 4s, 8s, 16s
		backoff := time.Duration(1<<uint(attempt)) * time.Second
		time.Sleep(backoff)
	}

	return nil, fmt.Errorf("max retries exceeded")
}

// CreatePrincipal creates a new principal (domain)
func (c *StalwartClient) CreatePrincipal(domain, description string) (int64, error) {
	body := map[string]interface{}{
		"type":        "domain",
		"name":        domain,
		"description": description,
	}

	resp, err := c.doRequest("POST", "/api/principal", body)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return 0, fmt.Errorf("error parsing response: %w", err)
	}

	// Check for error key
	if errorMsg, ok := result["error"]; ok {
		// If error is "fieldAlreadyExists", we need to get the ID
		if errorMsg == "fieldAlreadyExists" {
			// List principals to find the ID
			return c.GetPrincipalIDByName(domain)
		}
		return 0, fmt.Errorf("API error: %v", errorMsg)
	}

	// Extract ID from data field
	if data, ok := result["data"].(float64); ok {
		return int64(data), nil
	}

	return 0, fmt.Errorf("unexpected response format: %v", result)
}

// GetPrincipal gets a principal by ID
func (c *StalwartClient) GetPrincipal(id int64) (map[string]interface{}, error) {
	resp, err := c.doRequest("GET", fmt.Sprintf("/api/principal/%d", id), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("principal not found")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	// Check for API error
	if errorMsg, ok := result["error"].(string); ok {
		if errorMsg == "notFound" {
			return nil, fmt.Errorf("principal not found")
		}
		return nil, fmt.Errorf("API error: %v", errorMsg)
	}

	if data, ok := result["data"].(map[string]interface{}); ok {
		return data, nil
	}

	return nil, fmt.Errorf("unexpected response format, got: %v", result)
}

// UpdatePrincipal updates a domain principal using PATCH with action-based format
func (c *StalwartClient) UpdatePrincipal(id int64, domain, description string) error {
	// First, get the current state to determine what changed
	current, err := c.GetPrincipal(id)
	if err != nil {
		return fmt.Errorf("failed to get current principal state: %w", err)
	}

	// Build the patch actions array
	var actions []map[string]interface{}

	// Update name if changed
	if currentName, ok := current["name"].(string); ok && currentName != domain {
		actions = append(actions, map[string]interface{}{
			"action": "set",
			"field":  "name",
			"value":  domain,
		})
	}

	// Update description if changed
	if currentDesc, ok := current["description"].(string); ok && currentDesc != description {
		actions = append(actions, map[string]interface{}{
			"action": "set",
			"field":  "description",
			"value":  description,
		})
	}

	// If no changes, return early
	if len(actions) == 0 {
		return nil
	}

	resp, err := c.doRequest("PATCH", fmt.Sprintf("/api/principal/%d", id), actions)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("error parsing response: %w", err)
	}

	// Check for error key
	if errorMsg, ok := result["error"]; ok {
		return fmt.Errorf("API error: %v", errorMsg)
	}

	return nil
}

// DeletePrincipal deletes a principal
func (c *StalwartClient) DeletePrincipal(id int64) error {
	resp, err := c.doRequest("DELETE", fmt.Sprintf("/api/principal/%d", id), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// Already deleted, not an error
		return nil
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// GetPrincipalIDByName finds a principal ID by name
func (c *StalwartClient) GetPrincipalIDByName(name string) (int64, error) {
	resp, err := c.doRequest("GET", "/api/principal?types=domain", nil)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return 0, fmt.Errorf("error parsing response: %w", err)
	}

	if data, ok := result["data"].(map[string]interface{}); ok {
		if items, ok := data["items"].([]interface{}); ok {
			for _, item := range items {
				if itemMap, ok := item.(map[string]interface{}); ok {
					if itemMap["name"] == name {
						if id, ok := itemMap["id"].(float64); ok {
							return int64(id), nil
						}
					}
				}
			}
		}
	}

	return 0, fmt.Errorf("principal not found")
}

// GetGroupPrincipalIDByName finds a group principal ID by name
func (c *StalwartClient) GetGroupPrincipalIDByName(name string) (int64, error) {
	resp, err := c.doRequest("GET", "/api/principal?types=group", nil)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return 0, fmt.Errorf("error parsing response: %w", err)
	}

	if data, ok := result["data"].(map[string]interface{}); ok {
		if items, ok := data["items"].([]interface{}); ok {
			for _, item := range items {
				if itemMap, ok := item.(map[string]interface{}); ok {
					if itemMap["name"] == name {
						if id, ok := itemMap["id"].(float64); ok {
							return int64(id), nil
						}
					}
				}
			}
		}
	}

	return 0, fmt.Errorf("principal not found")
}

// GetAccountPrincipalIDByName finds an account principal ID by name
func (c *StalwartClient) GetAccountPrincipalIDByName(name string) (int64, error) {
	resp, err := c.doRequest("GET", "/api/principal?types=individual", nil)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return 0, fmt.Errorf("error parsing response: %w", err)
	}

	if data, ok := result["data"].(map[string]interface{}); ok {
		if items, ok := data["items"].([]interface{}); ok {
			for _, item := range items {
				if itemMap, ok := item.(map[string]interface{}); ok {
					if itemMap["name"] == name {
						if id, ok := itemMap["id"].(float64); ok {
							return int64(id), nil
						}
					}
				}
			}
		}
	}

	return 0, fmt.Errorf("principal not found")
}

// GetDNSRecords gets DNS records for a domain
func (c *StalwartClient) GetDNSRecords(domain string) ([]map[string]interface{}, error) {
	resp, err := c.doRequest("GET", fmt.Sprintf("/api/dns/records/%s", domain), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	if data, ok := result["data"].([]interface{}); ok {
		records := make([]map[string]interface{}, 0, len(data))
		for _, record := range data {
			if recordMap, ok := record.(map[string]interface{}); ok {
				records = append(records, recordMap)
			}
		}
		return records, nil
	}

	return nil, fmt.Errorf("unexpected response format")
}

// CreateDKIMKey creates a DKIM key for a domain (idempotent)
func (c *StalwartClient) CreateDKIMKey(domain, algorithm string) error {
	body := map[string]interface{}{
		"domain":    domain,
		"algorithm": algorithm,
	}

	resp, err := c.doRequest("POST", "/api/dkim", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Check if it's an "already exists" type error - if so, that's fine
		var result map[string]interface{}
		if json.Unmarshal(respBody, &result) == nil {
			if errorMsg, ok := result["error"].(string); ok {
				// If the DKIM key already exists, treat it as success
				if errorMsg == "fieldAlreadyExists" || errorMsg == "alreadyExists" {
					return nil
				}
			}
		}
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// CreateGroupPrincipal creates a new group principal
func (c *StalwartClient) CreateGroupPrincipal(name, description string, emails []string) (int64, error) {
	body := map[string]interface{}{
		"type":               "group",
		"name":               name,
		"description":        description,
		"emails":             emails,
		"enabledPermissions": []string{"email-send", "email-receive"},
	}

	resp, err := c.doRequest("POST", "/api/principal", body)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return 0, fmt.Errorf("error parsing response: %w", err)
	}

	// Check for error key
	if errorMsg, ok := result["error"]; ok {
		// If error is "fieldAlreadyExists", we need to get the ID
		if errorMsg == "fieldAlreadyExists" {
			// List principals to find the ID
			return c.GetGroupPrincipalIDByName(name)
		}
		return 0, fmt.Errorf("API error: %v (full response: %s)", errorMsg, string(respBody))
	}

	// Extract ID from data field
	if data, ok := result["data"].(float64); ok {
		return int64(data), nil
	}

	return 0, fmt.Errorf("unexpected response format: %v", result)
}

// UpdateGroupPrincipal updates a group principal using PATCH with action-based format
func (c *StalwartClient) UpdateGroupPrincipal(id int64, name, description string, emails []string) error {
	// First, get the current state to determine what changed
	current, err := c.GetPrincipal(id)
	if err != nil {
		return fmt.Errorf("failed to get current principal state: %w", err)
	}

	// Build the patch actions array
	var actions []map[string]interface{}

	// Update name if changed
	if currentName, ok := current["name"].(string); ok && currentName != name {
		actions = append(actions, map[string]interface{}{
			"action": "set",
			"field":  "name",
			"value":  name,
		})
	}

	// Update description if changed
	if currentDesc, ok := current["description"].(string); ok && currentDesc != description {
		actions = append(actions, map[string]interface{}{
			"action": "set",
			"field":  "description",
			"value":  description,
		})
	}

	// Update emails - get current emails (API can return string or array)
	var currentEmails []string
	switch v := current["emails"].(type) {
	case string:
		currentEmails = []string{v}
	case []interface{}:
		for _, e := range v {
			if emailStr, ok := e.(string); ok {
				currentEmails = append(currentEmails, emailStr)
			}
		}
	}

	// Find emails to add and remove
	currentEmailsSet := make(map[string]bool)
	for _, e := range currentEmails {
		currentEmailsSet[e] = true
	}

	newEmailsSet := make(map[string]bool)
	for _, e := range emails {
		newEmailsSet[e] = true
	}

	// Remove emails that are no longer present
	for _, email := range currentEmails {
		if !newEmailsSet[email] {
			actions = append(actions, map[string]interface{}{
				"action": "removeItem",
				"field":  "emails",
				"value":  email,
			})
		}
	}

	// Add new emails
	for _, email := range emails {
		if !currentEmailsSet[email] {
			actions = append(actions, map[string]interface{}{
				"action": "addItem",
				"field":  "emails",
				"value":  email,
			})
		}
	}

	// If no changes, return early
	if len(actions) == 0 {
		return nil
	}

	resp, err := c.doRequest("PATCH", fmt.Sprintf("/api/principal/%d", id), actions)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("error parsing response: %w", err)
	}

	// Check for error key
	if errorMsg, ok := result["error"]; ok {
		return fmt.Errorf("API error: %v", errorMsg)
	}

	return nil
}

// CreateAccountPrincipal creates a new account principal
func (c *StalwartClient) CreateAccountPrincipal(name, description string, emails []string, locale string, memberOf, roles, secrets []string) (int64, error) {
	body := map[string]interface{}{
		"type":               "individual",
		"name":               name,
		"description":        description,
		"emails":             emails,
		"locale":             locale,
		"quota":              0,
		"memberOf":           memberOf,           // Always send, can be empty array
		"roles":              roles,              // Always send (defaults to ["user"])
		"secrets":            secrets,            // Always send (required, at least 1)
		"enabledPermissions": []string{"email-send", "email-receive"}, // Default permissions
	}

	resp, err := c.doRequest("POST", "/api/principal", body)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return 0, fmt.Errorf("error parsing response: %w", err)
	}

	// Check for error key
	if errorMsg, ok := result["error"]; ok {
		// If error is "fieldAlreadyExists", we need to get the ID
		if errorMsg == "fieldAlreadyExists" {
			// List principals to find the ID
			return c.GetAccountPrincipalIDByName(name)
		}
		return 0, fmt.Errorf("API error: %v (full response: %s)", errorMsg, string(respBody))
	}

	// Extract ID from data field
	if data, ok := result["data"].(float64); ok {
		return int64(data), nil
	}

	return 0, fmt.Errorf("unexpected response format: %v", result)
}

// UpdateAccountPrincipal updates an account principal using PATCH with action-based format
func (c *StalwartClient) UpdateAccountPrincipal(id int64, name, description string, emails []string, locale string, memberOf, roles, secrets []string) error {
	// First, get the current state to determine what changed
	current, err := c.GetPrincipal(id)
	if err != nil {
		return fmt.Errorf("failed to get current principal state: %w", err)
	}

	// Build the patch actions array
	var actions []map[string]interface{}

	// Update name if changed
	if currentName, ok := current["name"].(string); ok && currentName != name {
		actions = append(actions, map[string]interface{}{
			"action": "set",
			"field":  "name",
			"value":  name,
		})
	}

	// Update description if changed
	if currentDesc, ok := current["description"].(string); ok && currentDesc != description {
		actions = append(actions, map[string]interface{}{
			"action": "set",
			"field":  "description",
			"value":  description,
		})
	}

	// Update locale if changed
	if currentLocale, ok := current["locale"].(string); ok && currentLocale != locale {
		actions = append(actions, map[string]interface{}{
			"action": "set",
			"field":  "locale",
			"value":  locale,
		})
	}

	// Helper function to get string array from interface (handles string or array)
	getStringArray := func(data interface{}) []string {
		var result []string
		switch v := data.(type) {
		case string:
			result = []string{v}
		case []interface{}:
			for _, item := range v {
				if str, ok := item.(string); ok {
					result = append(result, str)
				}
			}
		}
		return result
	}

	// Helper function to generate add/remove actions for array fields
	updateArrayField := func(fieldName string, currentItems, newItems []string) {
		currentSet := make(map[string]bool)
		for _, item := range currentItems {
			currentSet[item] = true
		}

		newSet := make(map[string]bool)
		for _, item := range newItems {
			newSet[item] = true
		}

		// Remove items that are no longer present
		for _, item := range currentItems {
			if !newSet[item] {
				actions = append(actions, map[string]interface{}{
					"action": "removeItem",
					"field":  fieldName,
					"value":  item,
				})
			}
		}

		// Add new items
		for _, item := range newItems {
			if !currentSet[item] {
				actions = append(actions, map[string]interface{}{
					"action": "addItem",
					"field":  fieldName,
					"value":  item,
				})
			}
		}
	}

	// Update emails
	currentEmails := getStringArray(current["emails"])
	updateArrayField("emails", currentEmails, emails)

	// Update memberOf (groups)
	currentMemberOf := getStringArray(current["memberOf"])
	updateArrayField("memberOf", currentMemberOf, memberOf)

	// Update roles
	currentRoles := getStringArray(current["roles"])
	updateArrayField("roles", currentRoles, roles)

	// Update secrets - NOTE: Secrets are hashed by API and returned as encrypted string
	// We cannot compare them, so we always "set" the entire secrets array
	// This is idempotent - sending the same secrets multiple times is safe
	actions = append(actions, map[string]interface{}{
		"action": "set",
		"field":  "secrets",
		"value":  secrets,
	})

	// If no changes, return early
	if len(actions) == 0 {
		return nil
	}

	resp, err := c.doRequest("PATCH", fmt.Sprintf("/api/principal/%d", id), actions)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("error parsing response: %w", err)
	}

	// Check for error key
	if errorMsg, ok := result["error"]; ok {
		return fmt.Errorf("API error: %v", errorMsg)
	}

	return nil
}
