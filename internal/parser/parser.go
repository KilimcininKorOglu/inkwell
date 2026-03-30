package parser

import (
	"context"
	"encoding/xml"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/KilimcininKorOglu/inkwell/internal/models"

	"gorm.io/gorm"
)

// ParseDMARCXML parses a single DMARC XML string and saves it to the database.
// domainID links the report to the source domain configuration (0 means unlinked).
func ParseDMARCXML(db *gorm.DB, xmlContent string, domainID uint) error {
	var feedback Feedback
	if err := xml.Unmarshal([]byte(xmlContent), &feedback); err != nil {
		return fmt.Errorf("failed to parse XML: %w", err)
	}

	meta := feedback.ReportMetadata
	policy := feedback.PolicyPublished

	// Idempotency check: skip if report already exists
	var existing models.Report
	if err := db.Where("report_id = ?", meta.ReportID).First(&existing).Error; err == nil {
		log.Printf("Report %s already parsed. Skipping.", meta.ReportID)
		return nil
	}

	// Convert Unix timestamps to time.Time
	beginDate := time.Unix(meta.DateRange.Begin, 0)
	endDate := time.Unix(meta.DateRange.End, 0)

	// Parse pct with default 100
	pct := 100
	if policy.Pct != "" {
		if v, err := strconv.Atoi(policy.Pct); err == nil {
			pct = v
		}
	}

	var domainIDPtr *uint
	if domainID > 0 {
		domainIDPtr = &domainID
	}

	report := models.Report{
		DomainID:  domainIDPtr,
		OrgName:   strPtr(meta.OrgName),
		Email:     strPtr(meta.Email),
		ReportID:  meta.ReportID,
		BeginDate: beginDate,
		EndDate:   endDate,
		Domain:    strPtr(policy.Domain),
		Adkim:     strPtr(policy.Adkim),
		Aspf:      strPtr(policy.Aspf),
		P:         strPtr(policy.P),
		Sp:        strPtr(policy.Sp),
		Pct:       &pct,
	}

	// Commit early to get report.ID for foreign keys
	if err := db.Create(&report).Error; err != nil {
		return fmt.Errorf("failed to insert report %s: %w", meta.ReportID, err)
	}

	for _, rec := range feedback.Records {
		row := rec.Row
		pe := row.PolicyEvaluated

		// Build reason string: join reason types with ", "
		var reasonParts []string
		for _, r := range pe.Reasons {
			if r.Type != "" {
				reasonParts = append(reasonParts, r.Type)
			}
		}
		reasonStr := strings.Join(reasonParts, ", ")

		headerFrom := rec.Identifiers.HeaderFrom

		// Reverse DNS with 1-second timeout
		var hostName *string
		if row.SourceIP != "" {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			names, err := net.DefaultResolver.LookupAddr(ctx, row.SourceIP)
			cancel()
			if err == nil && len(names) > 0 {
				// Remove trailing dot from reverse DNS result
				resolved := strings.TrimSuffix(names[0], ".")
				hostName = &resolved
			}
		}

		record := models.Record{
			ReportID:    report.ID,
			SourceIP:    strPtr(row.SourceIP),
			HostName:    hostName,
			Count:       row.Count,
			Disposition: strPtr(pe.Disposition),
			DKIM:        strPtr(pe.DKIM),
			SPF:         strPtr(pe.SPF),
			Reason:      strPtr(reasonStr),
			HeaderFrom:  strPtr(headerFrom),
		}

		// Per-record commit
		if err := db.Create(&record).Error; err != nil {
			log.Printf("Failed to insert record for IP %s: %v", row.SourceIP, err)
			continue
		}

		// DKIM auth results
		for _, dr := range rec.AuthResults.DKIM {
			ar := models.AuthResult{
				RecordID: record.ID,
				Type:     strPtr("dkim"),
				Domain:   strPtr(dr.Domain),
				Result:   strPtr(dr.Result),
				Selector: strPtr(dr.Selector),
			}
			if err := db.Create(&ar).Error; err != nil {
				log.Printf("Failed to insert DKIM auth result: %v", err)
			}
		}

		// SPF auth results
		for _, sr := range rec.AuthResults.SPF {
			ar := models.AuthResult{
				RecordID: record.ID,
				Type:     strPtr("spf"),
				Domain:   strPtr(sr.Domain),
				Result:   strPtr(sr.Result),
				Selector: nil, // SPF has no selector
			}
			if err := db.Create(&ar).Error; err != nil {
				log.Printf("Failed to insert SPF auth result: %v", err)
			}
		}
	}

	log.Printf("Successfully processed report %s", meta.ReportID)
	return nil
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
