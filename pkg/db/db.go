package db

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	DB   *gorm.DB
	once sync.Once
)

type Config struct {
	User      string
	Password  string
	Host      string
	Port      string
	DBName    string
	Charset   string // optional
	Loc       string // optional
	ParseTime bool   // optional
}

func NewConfig(user, password, host, port, dbName string) Config {
	return Config{
		User:      user,
		Password:  password,
		Host:      host,
		Port:      port,
		DBName:    dbName,
		Charset:   "utf8mb4",
		Loc:       "Local",
		ParseTime: true,
	}
}

func (cfg Config) DSN() string {

	charset := cfg.Charset
	if charset == "" {
		charset = "utf8mb4"
	}
	loc := cfg.Loc
	if loc == "" {
		loc = "Local"
	}
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s)/%s?charset=%s&parseTime=%t&loc=%s",
		cfg.User, cfg.Password, cfg.Host, cfg.DBName, charset, cfg.ParseTime, loc,
	)
	return dsn
}

func Init(cfg Config) *gorm.DB {

	//dsn := "user:password@tcp(127.0.0.1:3306)/edgeflow?charset=utf8mb4&parseTime=True&loc=Local"
	once.Do(func() {

		// 构造 DSN 字符串

		var err error
		DB, err = gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Warn),
		})
		if err != nil {
			log.Fatalf("failed to connect to database: %v", err)
		}

		// Set connection pool
		sqlDB, _ := DB.DB()
		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetMaxOpenConns(100)
		sqlDB.SetConnMaxLifetime(time.Hour)
	})
	return DB
}

func RunSQLFile(db *gorm.DB, path string) error {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var builder strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(string(bytes)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "--") || strings.HasPrefix(line, "#") {
			continue
		}
		builder.WriteString(line)
		builder.WriteByte('\n')
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	statements := strings.Split(builder.String(), ";")
	for _, stmt := range statements {
		query := strings.TrimSpace(stmt)
		if query == "" {
			continue
		}
		if err := db.Exec(query).Error; err != nil {
			return err
		}
	}
	return nil
}
