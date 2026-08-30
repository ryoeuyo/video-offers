package video

import (
	"net"
	"net/netip"
	"net/url"
	"regexp"
	"strings"

	"github.com/ruslan/video-offers/internal/domain"
)

var youtubeIDRe = regexp.MustCompile(`^[a-zA-Z0-9_-]{11}$`)

// ParsedURL — результат разбора и нормализации ссылки на видео.
type ParsedURL struct {
	Original   string
	Normalized string
	Provider   domain.Provider
	ExternalID string
}

// ParseURL валидирует ссылку (SSRF), определяет провайдера и нормализует URL.
func ParseURL(raw string) (ParsedURL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ParsedURL{}, domain.ErrInvalidInput.WithCode("validation_error", "url обязателен")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return ParsedURL{}, domain.ErrUnprocessable.WithCode("invalid_url", "невалидная ссылка").Wrap(err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return ParsedURL{}, domain.ErrUnprocessable.WithCode("invalid_url", "разрешены только http и https")
	}

	host := strings.ToLower(u.Hostname())
	if host == "" {
		return ParsedURL{}, domain.ErrUnprocessable.WithCode("invalid_url", "невалидная ссылка")
	}

	if err := checkSSRF(host); err != nil {
		return ParsedURL{}, err
	}

	switch providerHost(host) {
	case domain.ProviderYouTube:
		return parseYouTube(raw, u)
	case domain.ProviderTwitch:
		return parseGeneric(raw, u, domain.ProviderTwitch)
	case domain.ProviderVK:
		return parseGeneric(raw, u, domain.ProviderVK)
	default:
		return parseGeneric(raw, u, domain.ProviderOther)
	}
}

func parseYouTube(original string, u *url.URL) (ParsedURL, error) {
	id := extractYouTubeID(u)
	if id == "" {
		return ParsedURL{}, domain.ErrUnprocessable.WithCode("invalid_url", "не удалось извлечь id youtube-видео")
	}
	return ParsedURL{
		Original:   original,
		Normalized: "https://www.youtube.com/watch?v=" + id,
		Provider:   domain.ProviderYouTube,
		ExternalID: id,
	}, nil
}

func parseGeneric(original string, u *url.URL, provider domain.Provider) (ParsedURL, error) {
	normalized, err := normalizeGenericURL(u)
	if err != nil {
		return ParsedURL{}, err
	}
	return ParsedURL{
		Original:   original,
		Normalized: normalized,
		Provider:   provider,
		ExternalID: "",
	}, nil
}

func extractYouTubeID(u *url.URL) string {
	host := strings.ToLower(u.Hostname())
	q := u.Query()

	if id := q.Get("v"); youtubeIDRe.MatchString(id) {
		return id
	}

	path := strings.Trim(u.Path, "/")
	switch host {
	case "youtu.be":
		if parts := strings.Split(path, "/"); len(parts) >= 1 && youtubeIDRe.MatchString(parts[0]) {
			return parts[0]
		}
	case "youtube.com", "www.youtube.com", "m.youtube.com", "music.youtube.com":
		parts := strings.Split(path, "/")
		if len(parts) >= 2 {
			switch parts[0] {
			case "embed", "shorts", "live", "v":
				if youtubeIDRe.MatchString(parts[1]) {
					return parts[1]
				}
			}
		}
	}
	return ""
}

// normalizeGenericURL убирает utm-метки и фрагмент (#t=...).
func normalizeGenericURL(u *url.URL) (string, error) {
	clean := *u
	clean.Fragment = ""

	q := clean.Query()
	for key := range q {
		if strings.HasPrefix(strings.ToLower(key), "utm_") {
			q.Del(key)
		}
	}
	clean.RawQuery = q.Encode()

	if clean.Scheme == "" {
		clean.Scheme = "https"
	}
	return clean.String(), nil
}

func providerHost(host string) domain.Provider {
	switch host {
	case "youtube.com", "www.youtube.com", "m.youtube.com", "music.youtube.com", "youtu.be":
		return domain.ProviderYouTube
	case "twitch.tv", "www.twitch.tv", "clips.twitch.tv", "m.twitch.tv":
		return domain.ProviderTwitch
	case "vk.com", "www.vk.com", "vk.ru", "www.vk.ru", "vkvideo.ru", "www.vkvideo.ru":
		return domain.ProviderVK
	default:
		return domain.ProviderOther
	}
}

var trustedHosts = map[string]struct{}{
	"youtube.com": {}, "www.youtube.com": {}, "m.youtube.com": {}, "music.youtube.com": {}, "youtu.be": {},
	"twitch.tv": {}, "www.twitch.tv": {}, "clips.twitch.tv": {}, "m.twitch.tv": {},
	"vk.com": {}, "www.vk.com": {}, "vk.ru": {}, "www.vk.ru": {}, "vkvideo.ru": {}, "www.vkvideo.ru": {},
}

func checkSSRF(host string) error {
	if _, ok := trustedHosts[host]; ok {
		return nil
	}

	if ip, err := netip.ParseAddr(host); err == nil {
		if isPrivateIP(ip) {
			return domain.ErrUnprocessable.WithCode("invalid_url", "ссылка на запрещённый адрес")
		}
		return nil
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		return domain.ErrUnprocessable.WithCode("invalid_url", "не удалось проверить хост").Wrap(err)
	}
	for _, ip := range ips {
		addr, ok := netip.AddrFromSlice(ip)
		if !ok {
			continue
		}
		if isPrivateIP(addr) {
			return domain.ErrUnprocessable.WithCode("invalid_url", "ссылка на запрещённый адрес")
		}
	}
	return nil
}

func isPrivateIP(ip netip.Addr) bool {
	if !ip.IsValid() {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	// AWS/GCP metadata
	if ip == netip.MustParseAddr("169.254.169.254") {
		return true
	}
	return false
}
