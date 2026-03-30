package models

import (
	"log"
	"time"

	"gorm.io/gorm"
)

// Domain maps to the 'domains' table. Stores per-domain IMAP configuration.
type Domain struct {
	ID                uint   `gorm:"primaryKey;autoIncrement"`
	Name              string `gorm:"column:name;type:varchar(255);uniqueIndex"`
	IMAPServer        string `gorm:"column:imap_server;type:varchar(255)"`
	IMAPPort          int    `gorm:"column:imap_port;type:int;default:993"`
	IMAPUser          string `gorm:"column:imap_user;type:varchar(255)"`
	IMAPPassword      string `gorm:"column:imap_password;type:text"` // AES-256-GCM encrypted
	IMAPFolder        string `gorm:"column:imap_folder;type:varchar(255);default:INBOX"`
	IMAPMoveFolder    string `gorm:"column:imap_move_folder;type:varchar(255)"`
	IMAPMoveFolderErr string `gorm:"column:imap_move_folder_err;type:varchar(255)"`
	Enabled           bool   `gorm:"column:enabled;default:true"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (Domain) TableName() string { return "domains" }

// Report maps to the 'reports' table.
type Report struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	DomainID  *uint     `gorm:"column:domain_id;index"` // nullable for backward compat
	OrgName   *string   `gorm:"column:org_name;type:varchar(255)"`
	Email     *string   `gorm:"column:email;type:varchar(255)"`
	ReportID  string    `gorm:"column:report_id;type:varchar(255);uniqueIndex"`
	BeginDate time.Time `gorm:"column:begin_date"`
	EndDate   time.Time `gorm:"column:end_date"`
	Domain    *string   `gorm:"column:domain;type:varchar(255)"`
	Adkim     *string   `gorm:"column:adkim;type:varchar(10)"`
	Aspf      *string   `gorm:"column:aspf;type:varchar(10)"`
	P         *string   `gorm:"column:p;type:varchar(20)"`
	Sp        *string   `gorm:"column:sp;type:varchar(20)"`
	Pct       *int      `gorm:"column:pct;type:int"`
}

func (Report) TableName() string { return "reports" }

// Record maps to the 'records' table.
type Record struct {
	ID          uint    `gorm:"primaryKey;autoIncrement"`
	ReportID    uint    `gorm:"column:report_id;index"`
	SourceIP    *string `gorm:"column:source_ip;type:varchar(50)"`
	HostName    *string `gorm:"column:host_name;type:varchar(255)"`
	Count       int     `gorm:"column:count;type:int"`
	Disposition *string `gorm:"column:disposition;type:varchar(20)"`
	DKIM        *string `gorm:"column:dkim;type:varchar(20)"`
	SPF         *string `gorm:"column:spf;type:varchar(20)"`
	Reason      *string `gorm:"column:reason;type:varchar(255)"`
	HeaderFrom  *string `gorm:"column:header_from;type:varchar(255)"`
}

func (Record) TableName() string { return "records" }

// AuthResult maps to the 'auth_results' table.
type AuthResult struct {
	ID       uint    `gorm:"primaryKey;autoIncrement"`
	RecordID uint    `gorm:"column:record_id;index"`
	Type     *string `gorm:"column:type;type:varchar(10)"`
	Domain   *string `gorm:"column:domain;type:varchar(255)"`
	Result   *string `gorm:"column:result;type:varchar(20)"`
	Selector *string `gorm:"column:selector;type:varchar(255)"`
}

func (AuthResult) TableName() string { return "auth_results" }

// InitDB creates tables if they don't exist and runs migrations.
func InitDB(db *gorm.DB) {
	if err := db.AutoMigrate(&Domain{}, &Report{}, &Record{}, &AuthResult{}); err != nil {
		log.Printf("AutoMigrate error: %v", err)
	}
}
