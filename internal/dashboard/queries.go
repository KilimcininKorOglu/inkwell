package dashboard

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/KilimcininKorOglu/inkwell/internal/models"

	"gorm.io/gorm"
)

// HasAnyData checks if any reports exist.
func HasAnyData(db *gorm.DB) (bool, error) {
	var count int64
	if err := db.Model(&models.Report{}).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// FilterOptions holds the available filter values from the database.
type FilterOptions struct {
	MinDate time.Time
	MaxDate time.Time
	Domains []string
	Orgs    []string
}

// FetchFilterOptions returns min/max dates and distinct domains/orgs.
func FetchFilterOptions(db *gorm.DB) (*FilterOptions, error) {
	opts := &FilterOptions{}

	var result struct {
		MinDate *time.Time
		MaxDate *time.Time
	}
	if err := db.Model(&models.Report{}).
		Select("MIN(begin_date) as min_date, MAX(end_date) as max_date").
		Scan(&result).Error; err != nil {
		return opts, err
	}

	if result.MinDate != nil {
		opts.MinDate = *result.MinDate
	}
	if result.MaxDate != nil {
		opts.MaxDate = *result.MaxDate
	}

	if err := db.Model(&models.Report{}).
		Distinct("domain").
		Where("domain IS NOT NULL AND domain != ''").
		Pluck("domain", &opts.Domains).Error; err != nil {
		return opts, err
	}

	if err := db.Model(&models.Report{}).
		Distinct("org_name").
		Where("org_name IS NOT NULL AND org_name != ''").
		Pluck("org_name", &opts.Orgs).Error; err != nil {
		return opts, err
	}

	return opts, nil
}

// FetchGlobalMetrics returns the 3 top-level metrics for the given filters.
func FetchGlobalMetrics(db *gorm.DB, startDate, endDate time.Time, domains, orgs []string) (*MetricsData, error) {
	endDateInclusive := endDate.AddDate(0, 0, 1)

	var result struct {
		TotalIPs    int
		TotalVolume int64
		PassCount   int64
	}

	err := db.Table("records").
		Select(`COUNT(DISTINCT records.source_ip) as total_ips,
				COALESCE(SUM(records.count), 0) as total_volume,
				COALESCE(SUM(CASE WHEN records.disposition = 'none' THEN records.count ELSE 0 END), 0) as pass_count`).
		Joins("JOIN reports ON reports.id = records.report_id").
		Where("reports.domain IN ?", domains).
		Where("reports.org_name IN ?", orgs).
		Where("reports.begin_date >= ? AND reports.begin_date <= ?", startDate, endDateInclusive).
		Scan(&result).Error

	if err != nil {
		return nil, err
	}

	passRate := 0.0
	if result.TotalVolume > 0 {
		passRate = float64(result.PassCount) / float64(result.TotalVolume) * 100
	}

	return &MetricsData{
		TotalIPs:    result.TotalIPs,
		TotalVolume: result.TotalVolume,
		PassRateStr: fmt.Sprintf("%.1f%%", passRate),
	}, nil
}

// escapeLike escapes SQL LIKE metacharacters so they are treated as literals.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return s
}

// FetchReportsList returns the master report list, optionally filtered by search query.
func FetchReportsList(db *gorm.DB, startDate, endDate time.Time, domains, orgs []string, searchQuery string) ([]ReportRow, error) {
	endDateInclusive := endDate.AddDate(0, 0, 1)

	type rawRow struct {
		DBID     uint
		Begin    time.Time
		End      time.Time
		Domain   *string
		OrgName  *string
		ReportID string
		Messages int64
		Adkim    *string
		Aspf     *string
		P        *string
		Sp       *string
		Pct      *int
	}

	var rows []rawRow
	query := db.Table("reports").
		Select(`reports.id as db_id,
				reports.begin_date as "begin",
				reports.end_date as "end",
				reports.domain,
				reports.org_name,
				reports.report_id,
				COALESCE(SUM(records.count), 0) as messages,
				reports.adkim,
				reports.aspf,
				reports.p,
				reports.sp,
				reports.pct`).
		Joins("LEFT JOIN records ON reports.id = records.report_id").
		Where("reports.domain IN ?", domains).
		Where("reports.org_name IN ?", orgs).
		Where("reports.begin_date >= ? AND reports.begin_date <= ?", startDate, endDateInclusive)

	if searchQuery != "" {
		like := "%" + escapeLike(searchQuery) + "%"
		query = query.Where("(reports.domain LIKE ? OR reports.org_name LIKE ? OR reports.report_id LIKE ? OR records.source_ip LIKE ? OR records.host_name LIKE ?)",
			like, like, like, like, like)
	}

	err := query.Group("reports.id").
		Order("reports.begin_date DESC").
		Scan(&rows).Error

	if err != nil {
		return nil, err
	}

	var reports []ReportRow
	for _, r := range rows {
		reports = append(reports, ReportRow{
			DBID:         r.DBID,
			StartDate:    r.Begin.Format("2006-01-02 15:04"),
			EndDate:      r.End.Format("2006-01-02 15:04"),
			Domain:       derefStr(r.Domain),
			Organization: derefStr(r.OrgName),
			ReportID:     r.ReportID,
			Messages:     r.Messages,
			Adkim:        derefStr(r.Adkim),
			Aspf:         derefStr(r.Aspf),
			P:            derefStr(r.P),
			Sp:           derefStr(r.Sp),
			Pct:          derefInt(r.Pct),
		})
	}

	return reports, nil
}

// FetchReportDetail returns the full detail for a selected report.
func FetchReportDetail(db *gorm.DB, reportDBID uint) (*ReportDetailData, error) {
	// Get report metadata
	var report models.Report
	if err := db.First(&report, reportDBID).Error; err != nil {
		return nil, err
	}

	// Get all records for this report
	var records []models.Record
	db.Where("report_id = ?", reportDBID).Find(&records)

	if len(records) == 0 {
		return &ReportDetailData{
			DBID:         reportDBID,
			Organization: derefStr(report.OrgName),
			Domain:       derefStr(report.Domain),
			StartDate:    report.BeginDate.Format("2006-01-02 15:04"),
			EndDate:      report.EndDate.Format("2006-01-02 15:04"),
			Adkim:        derefStr(report.Adkim),
			Aspf:         derefStr(report.Aspf),
			P:            derefStr(report.P),
			Sp:           derefStr(report.Sp),
			Pct:          derefInt(report.Pct),
		}, nil
	}

	// Get all auth results for these records
	var recordIDs []uint
	for _, r := range records {
		recordIDs = append(recordIDs, r.ID)
	}

	var authResults []models.AuthResult
	db.Where("record_id IN ?", recordIDs).Find(&authResults)

	// Group auth results by record_id and type
	type authGroup struct {
		domains []string
		results []string
	}
	dkimByRecord := make(map[uint]*authGroup)
	spfByRecord := make(map[uint]*authGroup)

	for _, ar := range authResults {
		arType := derefStr(ar.Type)
		arDomain := derefStr(ar.Domain)
		arResult := derefStr(ar.Result)

		switch arType {
		case "dkim":
			if dkimByRecord[ar.RecordID] == nil {
				dkimByRecord[ar.RecordID] = &authGroup{}
			}
			if arDomain != "" {
				dkimByRecord[ar.RecordID].domains = append(dkimByRecord[ar.RecordID].domains, arDomain)
			}
			if arResult != "" {
				dkimByRecord[ar.RecordID].results = append(dkimByRecord[ar.RecordID].results, arResult)
			}
		case "spf":
			if spfByRecord[ar.RecordID] == nil {
				spfByRecord[ar.RecordID] = &authGroup{}
			}
			if arDomain != "" {
				spfByRecord[ar.RecordID].domains = append(spfByRecord[ar.RecordID].domains, arDomain)
			}
			if arResult != "" {
				spfByRecord[ar.RecordID].results = append(spfByRecord[ar.RecordID].results, arResult)
			}
		}
	}

	// Build detail records with auth data merged
	type detailKey struct {
		SourceIP    string
		HostName    string
		Disposition string
		Reason      string
		DKIMDomain  string
		DKIMAuth    string
		SPFDomain   string
		SPFAuth     string
		DKIMAlign   string
		SPFAlign    string
		DMARC       string
	}

	// IP stats aggregation: group by all columns, sum count
	aggregated := make(map[detailKey]int)

	for _, rec := range records {
		sourceIP := derefStr(rec.SourceIP)
		hostName := derefStr(rec.HostName)
		disposition := derefStr(rec.Disposition)
		reason := derefStr(rec.Reason)
		dkimAlign := derefStr(rec.DKIM)
		spfAlign := derefStr(rec.SPF)

		// DMARC derivation
		dmarc := "fail"
		if disposition == "none" {
			dmarc = "pass"
		} else if disposition == "" {
			dmarc = "unknown"
		}

		// Auth result aggregation (join with comma)
		var dkimDomain, dkimAuth, spfDomain, spfAuth string
		if dg := dkimByRecord[rec.ID]; dg != nil {
			dkimDomain = strings.Join(dg.domains, ", ")
			dkimAuth = strings.Join(dg.results, ", ")
		}
		if sg := spfByRecord[rec.ID]; sg != nil {
			spfDomain = strings.Join(sg.domains, ", ")
			spfAuth = strings.Join(sg.results, ", ")
		}

		key := detailKey{
			SourceIP:    sourceIP,
			HostName:    hostName,
			Disposition: disposition,
			Reason:      reason,
			DKIMDomain:  dkimDomain,
			DKIMAuth:    dkimAuth,
			SPFDomain:   spfDomain,
			SPFAuth:     spfAuth,
			DKIMAlign:   dkimAlign,
			SPFAlign:    spfAlign,
			DMARC:       dmarc,
		}
		aggregated[key] += rec.Count
	}

	// Convert to slice and sort by count descending
	var detailRecords []DetailRecord
	for key, count := range aggregated {
		detailRecords = append(detailRecords, DetailRecord{
			SourceIP:    key.SourceIP,
			HostName:    key.HostName,
			Count:       count,
			Disposition: key.Disposition,
			Reason:      key.Reason,
			DKIMDomain:  key.DKIMDomain,
			DKIMAuth:    key.DKIMAuth,
			SPFDomain:   key.SPFDomain,
			SPFAuth:     key.SPFAuth,
			DKIMAlign:   key.DKIMAlign,
			SPFAlign:    key.SPFAlign,
			DMARC:       key.DMARC,
		})
	}

	sort.Slice(detailRecords, func(i, j int) bool {
		return detailRecords[i].Count > detailRecords[j].Count
	})

	return &ReportDetailData{
		DBID:         reportDBID,
		Organization: derefStr(report.OrgName),
		Domain:       derefStr(report.Domain),
		StartDate:    report.BeginDate.Format("2006-01-02 15:04"),
		EndDate:      report.EndDate.Format("2006-01-02 15:04"),
		Adkim:        derefStr(report.Adkim),
		Aspf:         derefStr(report.Aspf),
		P:            derefStr(report.P),
		Sp:           derefStr(report.Sp),
		Pct:          derefInt(report.Pct),
		Records:      detailRecords,
	}, nil
}

// --- Domain CRUD Queries ---

// FetchAllDomains returns all domains for the management list.
func FetchAllDomains(db *gorm.DB) ([]DomainRow, error) {
	var domains []models.Domain
	if err := db.Order("name ASC").Find(&domains).Error; err != nil {
		return nil, err
	}

	var rows []DomainRow
	for _, d := range domains {
		rows = append(rows, DomainRow{
			ID:         d.ID,
			Name:       d.Name,
			IMAPServer: d.IMAPServer,
			IMAPUser:   d.IMAPUser,
			IMAPFolder: d.IMAPFolder,
			Enabled:    d.Enabled,
			CreatedAt:  d.CreatedAt.Format("2006-01-02 15:04"),
		})
	}
	return rows, nil
}

// FetchDomainByID returns a single domain by its ID.
func FetchDomainByID(db *gorm.DB, id uint) (*models.Domain, error) {
	var domain models.Domain
	if err := db.First(&domain, id).Error; err != nil {
		return nil, err
	}
	return &domain, nil
}

// CreateDomain inserts a new domain into the database.
func CreateDomain(db *gorm.DB, domain *models.Domain) error {
	return db.Create(domain).Error
}

// UpdateDomain saves changes to an existing domain.
func UpdateDomain(db *gorm.DB, domain *models.Domain) error {
	return db.Save(domain).Error
}

// DeleteDomain removes a domain by its ID.
func DeleteDomain(db *gorm.DB, id uint) error {
	return db.Delete(&models.Domain{}, id).Error
}

// ToggleDomain flips the enabled state of a domain.
func ToggleDomain(db *gorm.DB, id uint) error {
	return db.Exec("UPDATE domains SET enabled = NOT enabled WHERE id = ?", id).Error
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefInt(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}
