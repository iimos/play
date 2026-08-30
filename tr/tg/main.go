package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
	"golang.org/x/net/proxy"
)

// Config holds application configuration
type Config struct {
	AppID    int    // Telegram API ID
	AppHash  string // Telegram API Hash
	Phone    string // Your phone number
	Channel  string // Channel username to scrape
	Limit    int    // Max messages to fetch (0 for no limit)
	ProxyURL string // Optional proxy URL (e.g., "socks5://localhost:9050")
}

func main() {
	appID, _ := strconv.Atoi(os.Getenv("TG_API_ID"))
	appHash := os.Getenv("TG_API_HASH")
	// Get it from bot father.
	//token := os.Getenv("TG_BOT_TOKEN")

	// Configuration - replace with your values
	config := Config{
		AppID:    appID,          // Your API ID
		AppHash:  appHash,        // Your API Hash
		Phone:    "+XXXXXXXXXXX", // Your phone number
		Channel:  "newssmartlab", // Channel to scrape (without @)
		Limit:    100,            // Max messages to fetch
		ProxyURL: "",             // Optional proxy
	}

	// Create output directory
	if err := os.MkdirAll("telegram_data", 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Create CSV file
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("telegram_data/%s_%s.csv", config.Channel, timestamp)
	file, err := os.Create(filename)
	if err != nil {
		log.Fatalf("Failed to create CSV file: %v", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write CSV header
	if err := writer.Write([]string{
		"MessageID", "Date", "Views", "Text", "MediaType", "URLs",
	}); err != nil {
		log.Fatalf("Failed to write CSV header: %v", err)
	}

	homedir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Failed to get user home dir: %v", err)
	}

	// Setup Telegram client
	clientOpts := telegram.Options{
		SessionStorage: &telegram.FileSessionStorage{
			Path: homedir + "/tgbot-session.json",
		},
		//Logger:      zap.Must(zap.NewProduction()),
		DialTimeout: 5 * time.Second,
		MaxRetries:  3,
	}

	// Configure proxy if provided
	if config.ProxyURL != "" {
		dialer, err := proxy.SOCKS5("tcp", config.ProxyURL, nil, proxy.Direct)
		if err != nil {
			log.Fatalf("Failed to create proxy dialer: %v", err)
		}
		_ = dialer
		//clientOpts.Dialer = dialer
	}

	client := telegram.NewClient(appID, appHash, clientOpts)

	// Authentication flow
	flow := auth.NewFlow(
		auth.Constant(config.Phone, "", auth.CodeAuthenticatorFunc(
			func(ctx context.Context, sentCode *tg.AuthSentCode) (string, error) {
				fmt.Print("Enter code: ")
				var code string
				if _, err := fmt.Scan(&code); err != nil {
					return "", err
				}
				return code, nil
			},
		)),
		auth.SendCodeOptions{},
	)

	fmt.Println("client.Run...")
	ctx := context.Background()
	if err := client.Run(ctx, func(ctx context.Context) error {
		fmt.Println("Authenticate...")
		// Authenticate
		if err := client.Auth().IfNecessary(ctx, flow); err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}

		// Resolve channel
		fmt.Println("Resolve channel...")
		api := client.API()
		resolved, err := api.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{Username: config.Channel})
		if err != nil {
			return fmt.Errorf("failed to resolve channel: %w", err)
		}

		// Find the channel in resolved peers
		var channel *tg.Channel
		for _, peer := range resolved.GetChats() {
			if c, ok := peer.(*tg.Channel); ok {
				channel = c
				break
			}
		}

		if channel == nil {
			return fmt.Errorf("channel not found")
		}

		// Print channel info
		fmt.Printf("\nScraping data from: %s (@%s)\n", channel.Title, channel.Username)
		fmt.Printf("Channel ID: %d\n", channel.ID)
		//if channel.ParticipantsCount != 0 {
		fmt.Printf("Participants: %d\n", channel.ParticipantsCount)
		//}

		// Get messages
		req := &tg.MessagesGetHistoryRequest{
			Peer: &tg.InputPeerChannel{
				ChannelID:  channel.ID,
				AccessHash: channel.AccessHash,
			},
			Limit: config.Limit,
		}

		messages, err := api.MessagesGetHistory(ctx, req)
		if err != nil {
			return fmt.Errorf("failed to get messages: %w", err)
		}

		switch resp := messages.(type) {
		case *tg.MessagesChannelMessages:
			fmt.Printf("\nFound %d messages\n", len(resp.Messages))
			for _, msg := range resp.Messages {
				// Process each message
				if err := processMessage(writer, msg); err != nil {
					log.Printf("Error processing message: %v", err)
					continue
				}
			}
		default:
			return fmt.Errorf("unexpected response type: %T", messages)
		}

		fmt.Printf("\nFinished scraping! Saved data to %s\n", filename)
		return nil
	}); err != nil {
		log.Fatalf("Telegram client error: %v", err)
	}
}

// processMessage extracts data from a message and writes to CSV
func processMessage(writer *csv.Writer, msg tg.MessageClass) error {
	var (
		messageID int
		date      time.Time
		views     int
		text      string
		mediaType string
		urls      string
	)

	switch m := msg.(type) {
	case *tg.Message:
		messageID = m.ID
		date = time.Unix(int64(m.Date), 0)
		views = m.Views
		text = m.Message

		// Determine media type
		if m.Media != nil {
			switch m.Media.(type) {
			case *tg.MessageMediaPhoto:
				mediaType = "Photo"
			case *tg.MessageMediaDocument:
				mediaType = "Document"
			case *tg.MessageMediaWebPage:
				mediaType = "Web Page"
			default:
				mediaType = "Media"
			}
		} else {
			mediaType = "None"
		}

		// Extract URLs from entities
		if m.Entities != nil {
			// This is simplified - you'd want to properly extract URLs from entities
			urls = "Extracted URLs would go here"
		}

	case *tg.MessageService:
		messageID = m.ID
		date = time.Unix(int64(m.Date), 0)
		text = "Service message"
		mediaType = "Service"
	default:
		return fmt.Errorf("unsupported message type: %T", msg)
	}

	// Clean text for CSV
	text = sanitizeText(text)

	// Write to CSV
	return writer.Write([]string{
		strconv.Itoa(messageID),
		date.Format("2006-01-02 15:04:05"),
		strconv.Itoa(views),
		text,
		mediaType,
		urls,
	})
}

// sanitizeText prepares text for CSV output
func sanitizeText(text string) string {
	// Replace problematic characters
	return strings.ReplaceAll(strings.ReplaceAll(text, "\n", " "), "\r", "")
}
