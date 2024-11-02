package service

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/nedaZarei/Cloud_vocabularyService/config"
	"github.com/nedaZarei/Cloud_vocabularyService/db"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Service struct {
	cfg           *config.Config
	vocabDB       db.VocabDB
	e             *echo.Echo
	client        *http.Client
	totalRequests *prometheus.CounterVec
	redisHits     *prometheus.CounterVec
	errors        *prometheus.CounterVec
	latency       *prometheus.HistogramVec
}

func NewService(cfg *config.Config) *Service {
	// Create an HTTP client that skips TLS verification
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	// Define Prometheus metrics
	totalRequests := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_total_requests",
			Help: "Total number of requests for each API endpoint",
		},
		[]string{"endpoint"},
	)

	redisHits := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_redis_hits",
			Help: "Number of requests answered by Redis",
		},
		[]string{"endpoint"},
	)

	errors := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_errors",
			Help: "Number of unsuccessful responses",
		},
		[]string{"endpoint"},
	)

	latency := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "api_request_duration",
			Help:    "Histogram of request duration for each API endpoint in seconds",
			Buckets: prometheus.DefBuckets, // Default buckets for latency
		},
		[]string{"endpoint"},
	)

	prometheus.MustRegister(totalRequests)
	prometheus.MustRegister(redisHits)
	prometheus.MustRegister(errors)
	prometheus.MustRegister(latency)

	return &Service{
		e:             echo.New(),
		cfg:           cfg,
		client:        client,
		totalRequests: totalRequests,
		redisHits:     redisHits,
		errors:        errors,
		latency:       latency,
	}
}

func (s *Service) StartService() error {
	redisClient := db.InitRedisClient(s.cfg.Redis.Host, s.cfg.Redis.Port)
	if _, err := redisClient.Ping(context.Background()).Result(); err != nil {
		return fmt.Errorf("failed to connect to Redis: %v", err)
	}
	log.Println("Connected to Redis")
	s.vocabDB = db.NewVocabDB(redisClient)

	s.e.Use(middleware.Logger())
	s.e.Use(middleware.Recover())
	s.e.GET("/metrics", echo.WrapHandler(promhttp.Handler()))

	v1 := s.e.Group("/api/v1")
	v1.GET("/dictionary", s.dictionary)
	v1.GET("/randomword", s.randomword)

	if err := s.e.Start(s.cfg.Server.Port); err != nil {
		return fmt.Errorf("failed to start server: %v", err)
	}

	return nil
}

func (s *Service) dictionary(c echo.Context) error {
	start := time.Now()
	defer func() {
		duration := time.Since(start).Seconds()
		s.latency.WithLabelValues("/dictionary").Observe(duration) // Record latency
		s.totalRequests.WithLabelValues("/dictionary").Inc()
	}()

	word := c.QueryParam("word")
	if word == "" {
		s.errors.WithLabelValues("/dictionary").Inc()
		return c.String(http.StatusBadRequest, "No word provided")
	}

	meaning, err := s.vocabDB.GetVocab(c.Request().Context(), word)
	if err != nil {
		meaning, err = s.fetchAndCacheMeaning(c.Request().Context(), word)
		if err != nil {
			s.errors.WithLabelValues("/dictionary").Inc()
			return c.String(http.StatusInternalServerError, err.Error())
		}
		return c.String(http.StatusOK, "NINJA: "+meaning)
	}
	s.redisHits.WithLabelValues("/dictionary").Inc()
	return c.String(http.StatusOK, "REDIS: "+meaning)
}

func (s *Service) randomword(c echo.Context) error {
	start := time.Now()
	defer func() {
		duration := time.Since(start).Seconds()
		s.latency.WithLabelValues("/randomword").Observe(duration)
		s.totalRequests.WithLabelValues("/randomword").Inc()
	}()

	req, err := http.NewRequest("GET", s.cfg.Ninjas.WordGeneratorURL, nil)
	if err != nil {
		s.errors.WithLabelValues("/randomword").Inc()
		return fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Key", s.cfg.Ninjas.DefAPIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		s.errors.WithLabelValues("/randomword").Inc()
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	jsonData, err := io.ReadAll(resp.Body)
	if err != nil {
		s.errors.WithLabelValues("/randomword").Inc()
		return c.String(500, err.Error())
	}
	wordResponse := struct {
		Word []string `json:"word"`
	}{}
	if err := json.Unmarshal(jsonData, &wordResponse); err != nil {
		s.errors.WithLabelValues("/randomword").Inc()
		return c.String(http.StatusInternalServerError, err.Error())
	}

	meaning, err := s.vocabDB.GetVocab(c.Request().Context(), wordResponse.Word[0])
	if err != nil {
		meaning, err = s.fetchAndCacheMeaning(c.Request().Context(), wordResponse.Word[0])
		if err != nil {
			s.errors.WithLabelValues("/randomword").Inc()
			return c.String(http.StatusInternalServerError, err.Error())
		}
		return c.String(http.StatusOK, wordResponse.Word[0]+" is the word "+"NINJA: "+meaning)
	}
	s.redisHits.WithLabelValues("/randomword").Inc()
	return c.String(http.StatusOK, wordResponse.Word[0]+" is the word "+"REDIS: "+meaning)
}

// retrieve word meaning from external API and cache it
func (s *Service) fetchAndCacheMeaning(ctx context.Context, word string) (string, error) {
	req, err := http.NewRequest("GET", s.cfg.Ninjas.DefinitionURL+"?word="+word, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("X-API-Key", s.cfg.Ninjas.DefAPIKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	jsonData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	dictResponse := struct {
		Meaning string `json:"definition"`
	}{}
	if err := json.Unmarshal(jsonData, &dictResponse); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %v", err)
	}
	fmt.Println(string(jsonData))

	cacheTime := time.Duration(s.cfg.Redis.CacheTime) * time.Second
	if err := s.vocabDB.AddVocab(ctx, word, dictResponse.Meaning, cacheTime); err != nil {
		return "", fmt.Errorf("failed to cache meaning: %v", err)
	}

	return dictResponse.Meaning, nil
}
