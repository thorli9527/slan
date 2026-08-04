package repository

type gormCustomerRecord struct {
	CustomerID string `gorm:"primaryKey;size:64"`
	Email      string `gorm:"size:255;uniqueIndex"`
	Name       string `gorm:"size:255"`
	Country    string `gorm:"size:128"`
	Province   string `gorm:"size:128"`
	City       string `gorm:"size:128"`
	IPRegion   string `gorm:"size:128"`
	Status     string `gorm:"size:64;index"`
	CreatedAt  int64  `gorm:"not null"`
	UpdatedAt  int64  `gorm:"not null"`
}
