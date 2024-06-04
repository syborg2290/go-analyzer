package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"

	"github.com/PuerkitoBio/goquery"
	_ "github.com/go-sql-driver/mysql"
)

// Database configuration
const (
	DBUser     = "root"
	DBPassword = "syborg2290"
	DBName     = "shopify_analyzer"
)

// Check if a website is a Shopify store
func isShopifyStore(url string) (bool, error) {
	resp, err := http.Get(url)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	// Look for Shopify specific meta tags or scripts
	shopifyRegex := regexp.MustCompile(`shopify`)
	return shopifyRegex.Match(body), nil
}

// Fetch and scrape website content
func fetchWebsiteContent(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("failed to fetch website content, status code: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// Extract relevant information from HTML using goquery
func extractRelevantInfo(htmlContent string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewBufferString(htmlContent))
	if err != nil {
		return "", err
	}

	// Example: Extracting the title and description
	title := doc.Find("title").Text()
	description, _ := doc.Find("meta[name='description']").Attr("content")

	// Combine the extracted information into a single string
	storeData := fmt.Sprintf("Title: %s\nDescription: %s", title, description)
	return storeData, nil
}

// Define a struct for the OpenAI request
type OpenAIRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Define a struct for the OpenAI response
type OpenAIResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int      `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
}

type Choice struct {
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
}

// Function to analyze store using ChatGPT
func analyzeStore(storeData string) (string, error) {
	apiURL := "https://api.openai.com/v1/chat/completions"
	apiKey := "YOUR_OPENAI_API_KEY" // Replace with your actual API key

	// Create the request body
	messages := []Message{
		{"system", "You are a helpful assistant."},
		{"user", "Analyze the following Shopify store data and provide feedback: " + storeData},
	}
	requestBody := OpenAIRequest{
		Model:    "gpt-4",
		Messages: messages,
	}
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", err
	}

	// Make the HTTP request
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Read and parse the response
	var openAIResponse OpenAIResponse
	err = json.NewDecoder(resp.Body).Decode(&openAIResponse)
	if err != nil {
		return "", err
	}

	if len(openAIResponse.Choices) > 0 {
		return openAIResponse.Choices[0].Message.Content, nil
	}
	return "", fmt.Errorf("no response from OpenAI")
}

// Store Shopify store data in MySQL
func storeShopifyData(db *sql.DB, url string, isShopify bool, analysis string) (int64, error) {
	result, err := db.Exec("INSERT INTO stores (url, is_shopify, analysis) VALUES (?, ?, ?)", url, isShopify, analysis)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// Store scraped data in MySQL
func storeScrapedData(db *sql.DB, storeID int64, data string) error {
	_, err := db.Exec("INSERT INTO store_data (store_id, data) VALUES (?, ?)", storeID, data)
	return err
}

// Perform the full workflow
func analyzeShopifyStore(db *sql.DB, url string) {
	// Step 1: Identify if it's a Shopify store
	isShopify, err := isShopifyStore(url)
	if err != nil {
		fmt.Println("Error identifying store:", err)
		return
	}

	if !isShopify {
		fmt.Println("Not a Shopify store.")
		return
	}

	// Step 2: Fetch and scrape website content
	htmlContent, err := fetchWebsiteContent(url)
	if err != nil {
		fmt.Println("Error fetching website content:", err)
		return
	}

	print(htmlContent)

	storeData, err := extractRelevantInfo(htmlContent)
	if err != nil {
		fmt.Println("Error extracting relevant info:", err)
		return
	}

	// Step 3: Analyze the store
	analysis, err := analyzeStore(storeData)
	if err != nil {
		fmt.Println("Error analyzing store:", err)
		return
	}

	// Step 4: Store the data in MySQL
	storeID, err := storeShopifyData(db, url, isShopify, analysis)
	if err != nil {
		fmt.Println("Error storing store data:", err)
		return
	}

	err = storeScrapedData(db, storeID, storeData)
	if err != nil {
		fmt.Println("Error storing scraped data:", err)
	} else {
		fmt.Println("Data stored successfully.")
	}
}

func main() {
	// Connect to the MySQL database
	dsn := fmt.Sprintf("%s:%s@/%s", DBUser, DBPassword, DBName)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		fmt.Println("Error connecting to the database:", err)
		return
	}
	defer db.Close()

	// Example Shopify store URL
	url := ""
	analyzeShopifyStore(db, url)
}
