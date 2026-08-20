package utils

import (
	"fmt"
	"net/smtp"
	"os"

	"stok-servisi/config"
	"go.uber.org/zap"
)

// SendCriticalStockEmail depo yöneticisine kritik stok uyarı maili gönderir
func SendCriticalStockEmail(productName string, remainingStock int, productID uint) error {
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := os.Getenv("SMTP_PORT")
	senderEmail := os.Getenv("SMTP_USER")
	senderPassword := os.Getenv("SMTP_PASS")
	toEmail := os.Getenv("ALERT_RECEIVER_EMAIL")

	// SMTP ortam değişkenleri tanımlı değilse konsola log basıp geç
	if smtpHost == "" || senderEmail == "" || toEmail == "" {
		config.Logger.Warn("SMTP ayarları .env dosyasında bulunamadı, mail gönderimi atlandı")
		return nil
	}

	auth := smtp.PlainAuth("", senderEmail, senderPassword, smtpHost)

	subject := fmt.Sprintf("Subject: ⚠️ [KRİTİK STOK] %s Tükeniyor!\r\n", productName)
	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	body := fmt.Sprintf(`
		<h2 style="color: #d9534f;">⚠️ Kritik Stok Seviyesi Uyarısı</h2>
		<p>Aşağıdaki ürünün stoğu kritik seviyenin altına düşmüştür:</p>
		<ul>
			<li><strong>Ürün ID:</strong> %d</li>
			<li><strong>Ürün Adı:</strong> %s</li>
			<li><strong>Kalan Stok:</strong> <span style="color: red; font-size: 16px;">%d</span></li>
		</ul>
		<p>Lütfen en kısa sürede tedarik sürecini başlatınız.</p>
	`, productID, productName, remainingStock)

	msg := []byte(subject + mime + body)
	addr := fmt.Sprintf("%s:%s", smtpHost, smtpPort)

	err := smtp.SendMail(addr, auth, senderEmail, []string{toEmail}, msg)
	if err != nil {
		config.Logger.Error("E-posta gönderim hatası", zap.Error(err))
		return err
	}

	config.Logger.Info("✅ Kritik stok uyarı e-postası başarıyla gönderildi",
		zap.String("to", toEmail),
		zap.Uint("product_id", productID),
	)
	return nil
}