package fetcher

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/KilimcininKorOglu/inkwell/internal/models"
	"github.com/KilimcininKorOglu/inkwell/internal/validation"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message/mail"
)

// maxDecompressSize limits decompressed attachment size to prevent zip bomb DoS (100MB).
const maxDecompressSize = 100 * 1024 * 1024

// FetchDMARCReports connects to IMAP for a specific domain, finds unread emails, extracts XML reports.
// The domain.IMAPPassword must already be decrypted before calling this function.
func FetchDMARCReports(domain *models.Domain) ([]string, error) {
	if domain.IMAPServer == "" || domain.IMAPUser == "" || domain.IMAPPassword == "" {
		log.Printf("IMAP credentials not configured for domain %s.", domain.Name)
		return nil, nil
	}

	// SSRF protection: resolve IP once, skip private ranges, use IP directly
	publicIP, err := validation.ResolvePublicIP(domain.IMAPServer)
	if err != nil {
		log.Printf("SSRF blocked: %v for domain %s", err, domain.Name)
		return nil, fmt.Errorf("IMAP server %s validation failed: %w", domain.IMAPServer, err)
	}

	addr := fmt.Sprintf("%s:%d", publicIP, domain.IMAPPort)
	log.Printf("Connecting to IMAP %s (resolved from %s)...", addr, domain.IMAPServer)

	client, err := imapclient.DialTLS(addr, &imapclient.Options{
		TLSConfig: &tls.Config{},
	})
	if err != nil {
		return nil, fmt.Errorf("IMAP connect error: %w", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			log.Printf("IMAP close error: %v", err)
		}
	}()

	if err := client.Login(domain.IMAPUser, domain.IMAPPassword).Wait(); err != nil {
		return nil, fmt.Errorf("IMAP login error: %w", err)
	}

	selectData, err := client.Select(domain.IMAPFolder, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("failed to select folder '%s': %w", domain.IMAPFolder, err)
	}
	log.Printf("Successfully connected to '%s' (Total messages: %d)", domain.IMAPFolder, selectData.NumMessages)

	// Search for unread emails using UID
	searchCriteria := &imap.SearchCriteria{
		NotFlag: []imap.Flag{imap.FlagSeen},
	}
	searchData, err := client.UIDSearch(searchCriteria, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("IMAP search error: %w", err)
	}

	uids := searchData.AllUIDs()
	if len(uids) == 0 {
		log.Println("No unread DMARC emails found.")
		return nil, nil
	}

	log.Printf("Found %d unread messages.", len(uids))

	var xmlReports []string
	needsExpunge := false

	for _, uid := range uids {
		log.Printf("Processing message UID: %d", uid)

		uidSet := imap.UIDSet{}
		uidSet.AddNum(uid)

		fetchOptions := &imap.FetchOptions{
			BodySection: []*imap.FetchItemBodySection{{}},
		}

		messages := client.Fetch(uidSet, fetchOptions)
		msg := messages.Next()
		if msg == nil {
			messages.Close()
			continue
		}

		foundXML := false
		xmlsFromMsg, err := extractXMLFromMessage(msg)
		if err != nil {
			log.Printf("Error extracting from message UID %d: %v", uid, err)
		} else if len(xmlsFromMsg) > 0 {
			xmlReports = append(xmlReports, xmlsFromMsg...)
			foundXML = true
		}

		if err := messages.Close(); err != nil {
			log.Printf("Error closing fetch: %v", err)
		}

		// Determine destination folder
		destFolder := domain.IMAPMoveFolderErr
		if foundXML {
			destFolder = domain.IMAPMoveFolder
		}

		// Move message if destination folder is defined
		if destFolder != "" {
			// Try to create destination folder (idempotent)
			_ = client.Create(destFolder, nil).Wait()

			_, copyErr := client.Copy(uidSet, destFolder).Wait()
			if copyErr == nil {
				storeFlags := &imap.StoreFlags{
					Op:    imap.StoreFlagsAdd,
					Flags: []imap.Flag{imap.FlagDeleted},
				}
				storeCmd := client.Store(uidSet, storeFlags, nil)
				if err := storeCmd.Close(); err != nil {
					log.Printf("Error marking message %d as deleted: %v", uid, err)
				} else {
					needsExpunge = true
					log.Printf("Successfully copied message %d to %s and marked for deletion", uid, destFolder)
				}
			} else {
				log.Printf("Failed to copy message %d to %s: %v", uid, destFolder, copyErr)
			}
		}
	}

	// Expunge moved messages
	if needsExpunge {
		if _, err := client.Expunge().Collect(); err != nil {
			log.Printf("Expunge error: %v", err)
		} else {
			log.Println("Expunged moved messages from original folder.")
		}
	}

	if err := client.Logout().Wait(); err != nil {
		log.Printf("IMAP logout error: %v", err)
	}

	return xmlReports, nil
}

// extractXMLFromMessage walks MIME parts and extracts XML from attachments.
func extractXMLFromMessage(msg *imapclient.FetchMessageData) ([]string, error) {
	var xmlReports []string

	// Get the message body
	var bodyReader io.Reader
	for {
		item := msg.Next()
		if item == nil {
			break
		}
		section, ok := item.(imapclient.FetchItemDataBodySection)
		if ok {
			bodyReader = section.Literal
			break
		}
	}
	if bodyReader == nil {
		return nil, fmt.Errorf("no body section found")
	}

	mr, err := mail.CreateReader(bodyReader)
	if err != nil {
		return nil, fmt.Errorf("mail reader error: %w", err)
	}

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Error reading part: %v", err)
			continue
		}

		// Check if part is an attachment
		attachHeader, ok := part.Header.(*mail.AttachmentHeader)
		if !ok {
			continue
		}

		filename, _ := attachHeader.Filename()
		if filename == "" {
			continue
		}

		payload, err := io.ReadAll(io.LimitReader(part.Body, maxDecompressSize))
		if err != nil {
			log.Printf("Error reading attachment %s: %v", filename, err)
			continue
		}

		// Extract XML from ZIP, GZ, or raw XML
		extracted, err := extractXMLFromPayload(filename, payload)
		if err != nil {
			log.Printf("Error extracting attachment %s: %v", filename, err)
			continue
		}
		xmlReports = append(xmlReports, extracted...)
	}

	return xmlReports, nil
}

// extractXMLFromPayload handles .zip, .gz, and .xml files.
func extractXMLFromPayload(filename string, payload []byte) ([]string, error) {
	var results []string

	switch {
	case strings.HasSuffix(strings.ToLower(filename), ".zip"):
		reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
		if err != nil {
			return nil, fmt.Errorf("zip open error: %w", err)
		}
		for _, f := range reader.File {
			if !strings.HasSuffix(strings.ToLower(f.Name), ".xml") {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				log.Printf("Error opening zip entry %s: %v", f.Name, err)
				continue
			}
			data, err := io.ReadAll(io.LimitReader(rc, maxDecompressSize))
			rc.Close()
			if err != nil {
				log.Printf("Error reading zip entry %s: %v", f.Name, err)
				continue
			}
			content := toValidUTF8(data)
			results = append(results, content)
			log.Printf("Extracted XML from ZIP: %s", filename)
		}

	case strings.HasSuffix(strings.ToLower(filename), ".gz"):
		gr, err := gzip.NewReader(bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("gzip open error: %w", err)
		}
		data, err := io.ReadAll(io.LimitReader(gr, maxDecompressSize))
		gr.Close()
		if err != nil {
			return nil, fmt.Errorf("gzip read error: %w", err)
		}
		content := toValidUTF8(data)
		results = append(results, content)
		log.Printf("Extracted XML from GZ: %s", filename)

	case strings.HasSuffix(strings.ToLower(filename), ".xml"):
		content := toValidUTF8(payload)
		results = append(results, content)
		log.Printf("Found direct XML attachment: %s", filename)
	}

	return results, nil
}

// toValidUTF8 replaces invalid UTF-8 bytes with U+FFFD.
func toValidUTF8(data []byte) string {
	return strings.ToValidUTF8(string(data), "\uFFFD")
}
