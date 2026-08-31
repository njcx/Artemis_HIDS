package server

import (
	"bytes"
	"crypto/rand"
	"image"
	"image/color"
	"image/png"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang/freetype"
	"golang.org/x/image/font/gofont/goregular"
)

const (
	captchaWidth  = 120
	captchaHeight = 40
	captchaLength = 4
	captchaExpiry = 5 * time.Minute
)

// CaptchaStore 验证码存储
type CaptchaStore struct {
	mu    sync.RWMutex
	store map[string]*CaptchaItem
}

// CaptchaItem 验证码项
type CaptchaItem struct {
	Code      string
	ExpiresAt time.Time
}

var captchaStore = &CaptchaStore{
	store: make(map[string]*CaptchaItem),
}

// generateCaptchaCode 生成随机验证码
func generateCaptchaCode(length int) (string, error) {
	const charset = "0123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz"
	code := make([]byte, length)
	for i := range code {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		code[i] = charset[n.Int64()]
	}
	return string(code), nil
}

// generateCaptchaImage 生成验证码图片
func generateCaptchaImage(code string) ([]byte, error) {
	// 创建图片
	img := image.NewRGBA(image.Rect(0, 0, captchaWidth, captchaHeight))

	// 填充背景色
	bgColor := color.RGBA{R: 240, G: 240, B: 240, A: 255}
	for x := 0; x < captchaWidth; x++ {
		for y := 0; y < captchaHeight; y++ {
			img.Set(x, y, bgColor)
		}
	}

	// 添加干扰线
	for i := 0; i < 3; i++ {
		x1, _ := rand.Int(rand.Reader, big.NewInt(captchaWidth))
		y1, _ := rand.Int(rand.Reader, big.NewInt(captchaHeight))
		x2, _ := rand.Int(rand.Reader, big.NewInt(captchaWidth))
		y2, _ := rand.Int(rand.Reader, big.NewInt(captchaHeight))
		drawLine(img, int(x1.Int64()), int(y1.Int64()), int(x2.Int64()), int(y2.Int64()),
			color.RGBA{R: 200, G: 200, B: 200, A: 255})
	}

	// 加载字体
	fontData := goregular.TTF
	font, err := freetype.ParseFont(fontData)
	if err != nil {
		return nil, err
	}

	// 创建 freetype context
	c := freetype.NewContext()
	c.SetDPI(72)
	c.SetFont(font)
	c.SetFontSize(28)
	c.SetClip(img.Bounds())
	c.SetDst(img)

	// 绘制验证码文字
	spacing := captchaWidth / (len(code) + 1)
	for i, char := range code {
		// 随机颜色
		r, _ := rand.Int(rand.Reader, big.NewInt(100))
		g, _ := rand.Int(rand.Reader, big.NewInt(100))
		b, _ := rand.Int(rand.Reader, big.NewInt(100))
		textColor := color.RGBA{R: uint8(r.Int64()), G: uint8(g.Int64()), B: uint8(b.Int64()), A: 255}
		c.SetSrc(image.NewUniform(textColor))

		// 随机 Y 轴偏移
		yOffset, _ := rand.Int(rand.Reader, big.NewInt(5))
		pt := freetype.Pt(spacing*(i+1), 30+int(yOffset.Int64()))
		c.DrawString(string(char), pt)
	}

	// 添加干扰点
	for i := 0; i < 50; i++ {
		x, _ := rand.Int(rand.Reader, big.NewInt(captchaWidth))
		y, _ := rand.Int(rand.Reader, big.NewInt(captchaHeight))
		img.Set(int(x.Int64()), int(y.Int64()), color.RGBA{R: 100, G: 100, B: 100, A: 255})
	}

	// 将图片编码为 PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// drawLine 画线
func drawLine(img *image.RGBA, x1, y1, x2, y2 int, col color.Color) {
	dx := abs(x2 - x1)
	dy := abs(y2 - y1)
	sx, sy := 1, 1
	if x1 >= x2 {
		sx = -1
	}
	if y1 >= y2 {
		sy = -1
	}
	err := dx - dy

	for {
		img.Set(x1, y1, col)
		if x1 == x2 && y1 == y2 {
			return
		}
		e2 := err * 2
		if e2 > -dy {
			err -= dy
			x1 += sx
		}
		if e2 < dx {
			err += dx
			y1 += sy
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// SaveCaptcha 保存验证码
func (cs *CaptchaStore) SaveCaptcha(captchaID, code string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.store[captchaID] = &CaptchaItem{
		Code:      code,
		ExpiresAt: time.Now().Add(captchaExpiry),
	}
}

// VerifyCaptcha 验证验证码
func (cs *CaptchaStore) VerifyCaptcha(captchaID, code string) bool {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	item, exists := cs.store[captchaID]
	if !exists {
		return false
	}

	// 验证码已过期
	if time.Now().After(item.ExpiresAt) {
		delete(cs.store, captchaID)
		return false
	}

	// 验证码只能使用一次
	delete(cs.store, captchaID)

	// 不区分大小写比较
	return equalFold(item.Code, code)
}

// equalFold 不区分大小写比较
func equalFold(s, t string) bool {
	if len(s) != len(t) {
		return false
	}
	for i := 0; i < len(s); i++ {
		c1, c2 := s[i], t[i]
		if c1 >= 'A' && c1 <= 'Z' {
			c1 += 'a' - 'A'
		}
		if c2 >= 'A' && c2 <= 'Z' {
			c2 += 'a' - 'A'
		}
		if c1 != c2 {
			return false
		}
	}
	return true
}

// CleanupExpiredCaptchas 定期清理过期验证码
func (cs *CaptchaStore) CleanupExpiredCaptchas() {
	ticker := time.NewTicker(1 * time.Minute)
	go func() {
		for range ticker.C {
			cs.mu.Lock()
			now := time.Now()
			for id, item := range cs.store {
				if now.After(item.ExpiresAt) {
					delete(cs.store, id)
				}
			}
			cs.mu.Unlock()
		}
	}()
}

// CaptchaResponse 验证码响应
type CaptchaResponse struct {
	CaptchaID string `json:"captcha_id"`
}

// getCaptcha 获取验证码
func (s *APIServer) getCaptcha(c *gin.Context) {
	// 生成验证码
	code, err := generateCaptchaCode(captchaLength)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate captcha"})
		return
	}

	// 生成验证码ID
	captchaID, err := generateCaptchaCode(16)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate captcha ID"})
		return
	}

	// 保存验证码
	captchaStore.SaveCaptcha(captchaID, code)

	// 生成图片
	imgData, err := generateCaptchaImage(code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate captcha image"})
		return
	}

	// 返回图片和ID
	c.Header("X-Captcha-ID", captchaID)
	c.Data(http.StatusOK, "image/png", imgData)
}
