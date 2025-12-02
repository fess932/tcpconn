package tcpconn

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Statistics представляет статистику передачи данных
type Statistics struct {
	// Счётчики пакетов
	packetsSent     uint64
	packetsReceived uint64
	packetsLost     uint64
	packetsRetried  uint64

	// Счётчики байтов
	bytesSent     uint64
	bytesReceived uint64

	// Временные метки
	startTime     time.Time
	lastResetTime time.Time

	// Скорость передачи
	mu                    sync.RWMutex
	sendRateBytesPerSec   float64
	recvRateBytesPerSec   float64
	sendRatePacketsPerSec float64
	recvRatePacketsPerSec float64

	// История для расчёта скорости
	sendHistory []dataPoint
	recvHistory []dataPoint
	historySize int

	// Ошибки
	errors   uint64
	timeouts uint64
	resets   uint64

	// Задержки (в микросекундах)
	minLatency   uint64
	maxLatency   uint64
	avgLatency   uint64
	totalLatency uint64
	latencyCount uint64
}

type dataPoint struct {
	timestamp time.Time
	bytes     uint64
	packets   uint64
}

// NewStatistics создаёт новый экземпляр статистики
func NewStatistics() *Statistics {
	now := time.Now()
	return &Statistics{
		startTime:     now,
		lastResetTime: now,
		historySize:   60, // храним последние 60 секунд
		sendHistory:   make([]dataPoint, 0, 60),
		recvHistory:   make([]dataPoint, 0, 60),
		minLatency:    ^uint64(0), // максимальное значение uint64
	}
}

// RecordPacketSent записывает отправленный пакет
func (s *Statistics) RecordPacketSent(bytes uint64) {
	atomic.AddUint64(&s.packetsSent, 1)
	atomic.AddUint64(&s.bytesSent, bytes)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.sendHistory = append(s.sendHistory, dataPoint{
		timestamp: time.Now(),
		bytes:     bytes,
		packets:   1,
	})

	// Ограничиваем размер истории
	if len(s.sendHistory) > s.historySize {
		s.sendHistory = s.sendHistory[1:]
	}

	s.updateSendRate()
}

// RecordPacketReceived записывает полученный пакет
func (s *Statistics) RecordPacketReceived(bytes uint64) {
	atomic.AddUint64(&s.packetsReceived, 1)
	atomic.AddUint64(&s.bytesReceived, bytes)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.recvHistory = append(s.recvHistory, dataPoint{
		timestamp: time.Now(),
		bytes:     bytes,
		packets:   1,
	})

	if len(s.recvHistory) > s.historySize {
		s.recvHistory = s.recvHistory[1:]
	}

	s.updateRecvRate()
}

// RecordPacketLost записывает потерянный пакет
func (s *Statistics) RecordPacketLost() {
	atomic.AddUint64(&s.packetsLost, 1)
}

// RecordPacketRetried записывает повторную отправку пакета
func (s *Statistics) RecordPacketRetried() {
	atomic.AddUint64(&s.packetsRetried, 1)
}

// RecordError записывает ошибку
func (s *Statistics) RecordError() {
	atomic.AddUint64(&s.errors, 1)
}

// RecordTimeout записывает таймаут
func (s *Statistics) RecordTimeout() {
	atomic.AddUint64(&s.timeouts, 1)
}

// RecordReset записывает сброс соединения
func (s *Statistics) RecordReset() {
	atomic.AddUint64(&s.resets, 1)
}

// RecordLatency записывает задержку в микросекундах
func (s *Statistics) RecordLatency(latencyUs uint64) {
	// Обновляем минимум
	for {
		old := atomic.LoadUint64(&s.minLatency)
		if latencyUs >= old {
			break
		}
		if atomic.CompareAndSwapUint64(&s.minLatency, old, latencyUs) {
			break
		}
	}

	// Обновляем максимум
	for {
		old := atomic.LoadUint64(&s.maxLatency)
		if latencyUs <= old {
			break
		}
		if atomic.CompareAndSwapUint64(&s.maxLatency, old, latencyUs) {
			break
		}
	}

	// Обновляем среднее
	atomic.AddUint64(&s.totalLatency, latencyUs)
	count := atomic.AddUint64(&s.latencyCount, 1)
	atomic.StoreUint64(&s.avgLatency, atomic.LoadUint64(&s.totalLatency)/count)
}

// updateSendRate обновляет скорость отправки (должна вызываться под lock)
func (s *Statistics) updateSendRate() {
	if len(s.sendHistory) < 2 {
		return
	}

	// Берём данные за последние N секунд
	now := time.Now()
	cutoff := now.Add(-time.Second * 10) // последние 10 секунд

	var totalBytes uint64
	var totalPackets uint64
	var count int

	for i := len(s.sendHistory) - 1; i >= 0; i-- {
		if s.sendHistory[i].timestamp.Before(cutoff) {
			break
		}
		totalBytes += s.sendHistory[i].bytes
		totalPackets += s.sendHistory[i].packets
		count++
	}

	if count > 1 {
		duration := now.Sub(s.sendHistory[len(s.sendHistory)-count].timestamp).Seconds()
		if duration > 0 {
			s.sendRateBytesPerSec = float64(totalBytes) / duration
			s.sendRatePacketsPerSec = float64(totalPackets) / duration
		}
	}
}

// updateRecvRate обновляет скорость приёма (должна вызываться под lock)
func (s *Statistics) updateRecvRate() {
	if len(s.recvHistory) < 2 {
		return
	}

	now := time.Now()
	cutoff := now.Add(-time.Second * 10)

	var totalBytes uint64
	var totalPackets uint64
	var count int

	for i := len(s.recvHistory) - 1; i >= 0; i-- {
		if s.recvHistory[i].timestamp.Before(cutoff) {
			break
		}
		totalBytes += s.recvHistory[i].bytes
		totalPackets += s.recvHistory[i].packets
		count++
	}

	if count > 1 {
		duration := now.Sub(s.recvHistory[len(s.recvHistory)-count].timestamp).Seconds()
		if duration > 0 {
			s.recvRateBytesPerSec = float64(totalBytes) / duration
			s.recvRatePacketsPerSec = float64(totalPackets) / duration
		}
	}
}

// GetPacketsSent возвращает количество отправленных пакетов
func (s *Statistics) GetPacketsSent() uint64 {
	return atomic.LoadUint64(&s.packetsSent)
}

// GetPacketsReceived возвращает количество полученных пакетов
func (s *Statistics) GetPacketsReceived() uint64 {
	return atomic.LoadUint64(&s.packetsReceived)
}

// GetPacketsLost возвращает количество потерянных пакетов
func (s *Statistics) GetPacketsLost() uint64 {
	return atomic.LoadUint64(&s.packetsLost)
}

// GetPacketsRetried возвращает количество повторно отправленных пакетов
func (s *Statistics) GetPacketsRetried() uint64 {
	return atomic.LoadUint64(&s.packetsRetried)
}

// GetBytesSent возвращает количество отправленных байт
func (s *Statistics) GetBytesSent() uint64 {
	return atomic.LoadUint64(&s.bytesSent)
}

// GetBytesReceived возвращает количество полученных байт
func (s *Statistics) GetBytesReceived() uint64 {
	return atomic.LoadUint64(&s.bytesReceived)
}

// GetErrors возвращает количество ошибок
func (s *Statistics) GetErrors() uint64 {
	return atomic.LoadUint64(&s.errors)
}

// GetTimeouts возвращает количество таймаутов
func (s *Statistics) GetTimeouts() uint64 {
	return atomic.LoadUint64(&s.timeouts)
}

// GetResets возвращает количество сбросов
func (s *Statistics) GetResets() uint64 {
	return atomic.LoadUint64(&s.resets)
}

// GetSendRate возвращает скорость отправки в байтах/сек
func (s *Statistics) GetSendRate() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sendRateBytesPerSec
}

// GetRecvRate возвращает скорость приёма в байтах/сек
func (s *Statistics) GetRecvRate() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.recvRateBytesPerSec
}

// GetSendRatePackets возвращает скорость отправки в пакетах/сек
func (s *Statistics) GetSendRatePackets() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sendRatePacketsPerSec
}

// GetRecvRatePackets возвращает скорость приёма в пакетах/сек
func (s *Statistics) GetRecvRatePackets() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.recvRatePacketsPerSec
}

// GetMinLatency возвращает минимальную задержку в микросекундах
func (s *Statistics) GetMinLatency() uint64 {
	lat := atomic.LoadUint64(&s.minLatency)
	if lat == ^uint64(0) {
		return 0
	}
	return lat
}

// GetMaxLatency возвращает максимальную задержку в микросекундах
func (s *Statistics) GetMaxLatency() uint64 {
	return atomic.LoadUint64(&s.maxLatency)
}

// GetAvgLatency возвращает среднюю задержку в микросекундах
func (s *Statistics) GetAvgLatency() uint64 {
	return atomic.LoadUint64(&s.avgLatency)
}

// GetPacketLossRate возвращает процент потерянных пакетов
func (s *Statistics) GetPacketLossRate() float64 {
	sent := atomic.LoadUint64(&s.packetsSent)
	lost := atomic.LoadUint64(&s.packetsLost)

	if sent == 0 {
		return 0.0
	}

	return float64(lost) / float64(sent) * 100.0
}

// GetUptime возвращает время работы
func (s *Statistics) GetUptime() time.Duration {
	return time.Since(s.startTime)
}

// GetTimeSinceReset возвращает время с последнего сброса
func (s *Statistics) GetTimeSinceReset() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return time.Since(s.lastResetTime)
}

// Reset сбрасывает всю статистику
func (s *Statistics) Reset() {
	atomic.StoreUint64(&s.packetsSent, 0)
	atomic.StoreUint64(&s.packetsReceived, 0)
	atomic.StoreUint64(&s.packetsLost, 0)
	atomic.StoreUint64(&s.packetsRetried, 0)
	atomic.StoreUint64(&s.bytesSent, 0)
	atomic.StoreUint64(&s.bytesReceived, 0)
	atomic.StoreUint64(&s.errors, 0)
	atomic.StoreUint64(&s.timeouts, 0)
	atomic.StoreUint64(&s.resets, 0)
	atomic.StoreUint64(&s.minLatency, ^uint64(0))
	atomic.StoreUint64(&s.maxLatency, 0)
	atomic.StoreUint64(&s.avgLatency, 0)
	atomic.StoreUint64(&s.totalLatency, 0)
	atomic.StoreUint64(&s.latencyCount, 0)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastResetTime = time.Now()
	s.sendHistory = make([]dataPoint, 0, s.historySize)
	s.recvHistory = make([]dataPoint, 0, s.historySize)
	s.sendRateBytesPerSec = 0
	s.recvRateBytesPerSec = 0
	s.sendRatePacketsPerSec = 0
	s.recvRatePacketsPerSec = 0
}

// Snapshot представляет снимок статистики в определённый момент времени
type Snapshot struct {
	Timestamp time.Time

	// Счётчики
	PacketsSent     uint64
	PacketsReceived uint64
	PacketsLost     uint64
	PacketsRetried  uint64
	BytesSent       uint64
	BytesReceived   uint64
	Errors          uint64
	Timeouts        uint64
	Resets          uint64

	// Скорости
	SendRateBytesPerSec   float64
	RecvRateBytesPerSec   float64
	SendRatePacketsPerSec float64
	RecvRatePacketsPerSec float64

	// Задержки (микросекунды)
	MinLatencyUs uint64
	MaxLatencyUs uint64
	AvgLatencyUs uint64

	// Производные метрики
	PacketLossRate float64
	Uptime         time.Duration
	TimeSinceReset time.Duration
}

// GetSnapshot возвращает снимок текущей статистики
func (s *Statistics) GetSnapshot() Snapshot {
	return Snapshot{
		Timestamp:             time.Now(),
		PacketsSent:           s.GetPacketsSent(),
		PacketsReceived:       s.GetPacketsReceived(),
		PacketsLost:           s.GetPacketsLost(),
		PacketsRetried:        s.GetPacketsRetried(),
		BytesSent:             s.GetBytesSent(),
		BytesReceived:         s.GetBytesReceived(),
		Errors:                s.GetErrors(),
		Timeouts:              s.GetTimeouts(),
		Resets:                s.GetResets(),
		SendRateBytesPerSec:   s.GetSendRate(),
		RecvRateBytesPerSec:   s.GetRecvRate(),
		SendRatePacketsPerSec: s.GetSendRatePackets(),
		RecvRatePacketsPerSec: s.GetRecvRatePackets(),
		MinLatencyUs:          s.GetMinLatency(),
		MaxLatencyUs:          s.GetMaxLatency(),
		AvgLatencyUs:          s.GetAvgLatency(),
		PacketLossRate:        s.GetPacketLossRate(),
		Uptime:                s.GetUptime(),
		TimeSinceReset:        s.GetTimeSinceReset(),
	}
}

// FormatBytes форматирует байты в читаемый вид
func FormatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// FormatRate форматирует скорость в читаемый вид
func FormatRate(bytesPerSec float64) string {
	return fmt.Sprintf("%s/s", FormatBytes(uint64(bytesPerSec)))
}

// String возвращает строковое представление статистики
func (snap Snapshot) String() string {
	return fmt.Sprintf(`Statistics Snapshot:
  Uptime: %v (since reset: %v)

  Packets:
    Sent:     %d (%.2f pkt/s)
    Received: %d (%.2f pkt/s)
    Lost:     %d (%.2f%%)
    Retried:  %d

  Bytes:
    Sent:     %s (%s)
    Received: %s (%s)

  Errors:
    Errors:   %d
    Timeouts: %d
    Resets:   %d

  Latency:
    Min: %d μs
    Avg: %d μs
    Max: %d μs`,
		snap.Uptime, snap.TimeSinceReset,
		snap.PacketsSent, snap.SendRatePacketsPerSec,
		snap.PacketsReceived, snap.RecvRatePacketsPerSec,
		snap.PacketsLost, snap.PacketLossRate,
		snap.PacketsRetried,
		FormatBytes(snap.BytesSent), FormatRate(snap.SendRateBytesPerSec),
		FormatBytes(snap.BytesReceived), FormatRate(snap.RecvRateBytesPerSec),
		snap.Errors, snap.Timeouts, snap.Resets,
		snap.MinLatencyUs, snap.AvgLatencyUs, snap.MaxLatencyUs,
	)
}
