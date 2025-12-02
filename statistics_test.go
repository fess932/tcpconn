package tcpconn

import (
	"testing"
	"time"
)

func TestNewStatistics(t *testing.T) {
	stats := NewStatistics()
	if stats == nil {
		t.Fatal("NewStatistics() returned nil")
	}

	if stats.GetPacketsSent() != 0 {
		t.Errorf("GetPacketsSent() = %v, want 0", stats.GetPacketsSent())
	}

	if stats.GetPacketsReceived() != 0 {
		t.Errorf("GetPacketsReceived() = %v, want 0", stats.GetPacketsReceived())
	}

	if stats.GetUptime() < 0 {
		t.Error("GetUptime() returned negative duration")
	}
}

func TestStatistics_RecordPacketSent(t *testing.T) {
	stats := NewStatistics()

	stats.RecordPacketSent(100)
	stats.RecordPacketSent(200)
	stats.RecordPacketSent(300)

	if got := stats.GetPacketsSent(); got != 3 {
		t.Errorf("GetPacketsSent() = %v, want 3", got)
	}

	if got := stats.GetBytesSent(); got != 600 {
		t.Errorf("GetBytesSent() = %v, want 600", got)
	}
}

func TestStatistics_RecordPacketReceived(t *testing.T) {
	stats := NewStatistics()

	stats.RecordPacketReceived(150)
	stats.RecordPacketReceived(250)

	if got := stats.GetPacketsReceived(); got != 2 {
		t.Errorf("GetPacketsReceived() = %v, want 2", got)
	}

	if got := stats.GetBytesReceived(); got != 400 {
		t.Errorf("GetBytesReceived() = %v, want 400", got)
	}
}

func TestStatistics_RecordPacketLost(t *testing.T) {
	stats := NewStatistics()

	stats.RecordPacketLost()
	stats.RecordPacketLost()
	stats.RecordPacketLost()

	if got := stats.GetPacketsLost(); got != 3 {
		t.Errorf("GetPacketsLost() = %v, want 3", got)
	}
}

func TestStatistics_RecordPacketRetried(t *testing.T) {
	stats := NewStatistics()

	stats.RecordPacketRetried()
	stats.RecordPacketRetried()

	if got := stats.GetPacketsRetried(); got != 2 {
		t.Errorf("GetPacketsRetried() = %v, want 2", got)
	}
}

func TestStatistics_RecordError(t *testing.T) {
	stats := NewStatistics()

	stats.RecordError()
	stats.RecordError()
	stats.RecordError()

	if got := stats.GetErrors(); got != 3 {
		t.Errorf("GetErrors() = %v, want 3", got)
	}
}

func TestStatistics_RecordTimeout(t *testing.T) {
	stats := NewStatistics()

	stats.RecordTimeout()

	if got := stats.GetTimeouts(); got != 1 {
		t.Errorf("GetTimeouts() = %v, want 1", got)
	}
}

func TestStatistics_RecordReset(t *testing.T) {
	stats := NewStatistics()

	stats.RecordReset()
	stats.RecordReset()

	if got := stats.GetResets(); got != 2 {
		t.Errorf("GetResets() = %v, want 2", got)
	}
}

func TestStatistics_RecordLatency(t *testing.T) {
	stats := NewStatistics()

	stats.RecordLatency(100)
	stats.RecordLatency(200)
	stats.RecordLatency(150)

	min := stats.GetMinLatency()
	if min != 100 {
		t.Errorf("GetMinLatency() = %v, want 100", min)
	}

	max := stats.GetMaxLatency()
	if max != 200 {
		t.Errorf("GetMaxLatency() = %v, want 200", max)
	}

	avg := stats.GetAvgLatency()
	expected := uint64((100 + 200 + 150) / 3)
	if avg != expected {
		t.Errorf("GetAvgLatency() = %v, want %v", avg, expected)
	}
}

func TestStatistics_GetPacketLossRate(t *testing.T) {
	stats := NewStatistics()

	// Отправляем 100 пакетов
	for i := 0; i < 100; i++ {
		stats.RecordPacketSent(100)
	}

	// Теряем 10 пакетов
	for i := 0; i < 10; i++ {
		stats.RecordPacketLost()
	}

	lossRate := stats.GetPacketLossRate()
	expected := 10.0 // 10%

	if lossRate != expected {
		t.Errorf("GetPacketLossRate() = %.2f, want %.2f", lossRate, expected)
	}
}

func TestStatistics_GetPacketLossRateZeroSent(t *testing.T) {
	stats := NewStatistics()

	stats.RecordPacketLost()

	lossRate := stats.GetPacketLossRate()
	if lossRate != 0.0 {
		t.Errorf("GetPacketLossRate() with zero sent = %.2f, want 0.0", lossRate)
	}
}

func TestStatistics_SendRate(t *testing.T) {
	stats := NewStatistics()

	// Отправляем пакеты с небольшими задержками
	for i := 0; i < 10; i++ {
		stats.RecordPacketSent(1000)
		time.Sleep(10 * time.Millisecond)
	}

	// Даём время на расчёт скорости
	time.Sleep(100 * time.Millisecond)

	rate := stats.GetSendRate()
	if rate <= 0 {
		t.Errorf("GetSendRate() = %.2f, want > 0", rate)
	}

	ratePackets := stats.GetSendRatePackets()
	if ratePackets <= 0 {
		t.Errorf("GetSendRatePackets() = %.2f, want > 0", ratePackets)
	}
}

func TestStatistics_RecvRate(t *testing.T) {
	stats := NewStatistics()

	// Получаем пакеты
	for i := 0; i < 10; i++ {
		stats.RecordPacketReceived(1000)
		time.Sleep(10 * time.Millisecond)
	}

	time.Sleep(100 * time.Millisecond)

	rate := stats.GetRecvRate()
	if rate <= 0 {
		t.Errorf("GetRecvRate() = %.2f, want > 0", rate)
	}

	ratePackets := stats.GetRecvRatePackets()
	if ratePackets <= 0 {
		t.Errorf("GetRecvRatePackets() = %.2f, want > 0", ratePackets)
	}
}

func TestStatistics_Reset(t *testing.T) {
	stats := NewStatistics()

	// Записываем некоторые данные
	stats.RecordPacketSent(100)
	stats.RecordPacketReceived(200)
	stats.RecordPacketLost()
	stats.RecordError()
	stats.RecordLatency(100)

	// Проверяем, что данные записались
	if stats.GetPacketsSent() != 1 {
		t.Error("Data should be recorded before reset")
	}

	// Сбрасываем
	stats.Reset()

	// Проверяем, что всё обнулилось
	if got := stats.GetPacketsSent(); got != 0 {
		t.Errorf("After reset GetPacketsSent() = %v, want 0", got)
	}

	if got := stats.GetPacketsReceived(); got != 0 {
		t.Errorf("After reset GetPacketsReceived() = %v, want 0", got)
	}

	if got := stats.GetPacketsLost(); got != 0 {
		t.Errorf("After reset GetPacketsLost() = %v, want 0", got)
	}

	if got := stats.GetBytesSent(); got != 0 {
		t.Errorf("After reset GetBytesSent() = %v, want 0", got)
	}

	if got := stats.GetBytesReceived(); got != 0 {
		t.Errorf("After reset GetBytesReceived() = %v, want 0", got)
	}

	if got := stats.GetErrors(); got != 0 {
		t.Errorf("After reset GetErrors() = %v, want 0", got)
	}

	if got := stats.GetMinLatency(); got != 0 {
		t.Errorf("After reset GetMinLatency() = %v, want 0", got)
	}

	if got := stats.GetMaxLatency(); got != 0 {
		t.Errorf("After reset GetMaxLatency() = %v, want 0", got)
	}

	if got := stats.GetAvgLatency(); got != 0 {
		t.Errorf("After reset GetAvgLatency() = %v, want 0", got)
	}
}

func TestStatistics_GetSnapshot(t *testing.T) {
	stats := NewStatistics()

	stats.RecordPacketSent(1000)
	stats.RecordPacketReceived(2000)
	stats.RecordPacketLost()
	stats.RecordError()
	stats.RecordLatency(100)

	snapshot := stats.GetSnapshot()

	if snapshot.PacketsSent != 1 {
		t.Errorf("Snapshot.PacketsSent = %v, want 1", snapshot.PacketsSent)
	}

	if snapshot.PacketsReceived != 1 {
		t.Errorf("Snapshot.PacketsReceived = %v, want 1", snapshot.PacketsReceived)
	}

	if snapshot.PacketsLost != 1 {
		t.Errorf("Snapshot.PacketsLost = %v, want 1", snapshot.PacketsLost)
	}

	if snapshot.BytesSent != 1000 {
		t.Errorf("Snapshot.BytesSent = %v, want 1000", snapshot.BytesSent)
	}

	if snapshot.BytesReceived != 2000 {
		t.Errorf("Snapshot.BytesReceived = %v, want 2000", snapshot.BytesReceived)
	}

	if snapshot.Errors != 1 {
		t.Errorf("Snapshot.Errors = %v, want 1", snapshot.Errors)
	}

	if snapshot.MinLatencyUs != 100 {
		t.Errorf("Snapshot.MinLatencyUs = %v, want 100", snapshot.MinLatencyUs)
	}
}

func TestStatistics_Uptime(t *testing.T) {
	stats := NewStatistics()

	time.Sleep(100 * time.Millisecond)

	uptime := stats.GetUptime()
	if uptime < 100*time.Millisecond {
		t.Errorf("GetUptime() = %v, want >= 100ms", uptime)
	}
}

func TestStatistics_TimeSinceReset(t *testing.T) {
	stats := NewStatistics()

	time.Sleep(50 * time.Millisecond)

	stats.Reset()

	time.Sleep(50 * time.Millisecond)

	timeSinceReset := stats.GetTimeSinceReset()
	if timeSinceReset < 50*time.Millisecond {
		t.Errorf("GetTimeSinceReset() = %v, want >= 50ms", timeSinceReset)
	}

	if timeSinceReset > 100*time.Millisecond {
		t.Errorf("GetTimeSinceReset() = %v, want < 100ms", timeSinceReset)
	}
}

func TestStatistics_ConcurrentAccess(t *testing.T) {
	stats := NewStatistics()

	// Запускаем несколько горутин, которые одновременно записывают статистику
	done := make(chan bool)

	// Отправители
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				stats.RecordPacketSent(100)
			}
			done <- true
		}()
	}

	// Получатели
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				stats.RecordPacketReceived(100)
			}
			done <- true
		}()
	}

	// Читатели
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				stats.GetSnapshot()
			}
			done <- true
		}()
	}

	// Ждём завершения всех горутин
	for i := 0; i < 25; i++ {
		<-done
	}

	// Проверяем, что все пакеты записались
	if got := stats.GetPacketsSent(); got != 1000 {
		t.Errorf("After concurrent writes GetPacketsSent() = %v, want 1000", got)
	}

	if got := stats.GetPacketsReceived(); got != 1000 {
		t.Errorf("After concurrent writes GetPacketsReceived() = %v, want 1000", got)
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes uint64
		want  string
	}{
		{0, "0 B"},
		{100, "100 B"},
		{1023, "1023 B"},
		{1024, "1.00 KiB"},
		{1536, "1.50 KiB"},
		{1024 * 1024, "1.00 MiB"},
		{1024 * 1024 * 1024, "1.00 GiB"},
		{1024 * 1024 * 1024 * 1024, "1.00 TiB"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := FormatBytes(tt.bytes)
			if got != tt.want {
				t.Errorf("FormatBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestFormatRate(t *testing.T) {
	rate := FormatRate(1024 * 1024) // 1 MiB/s
	expected := "1.00 MiB/s"

	if rate != expected {
		t.Errorf("FormatRate(1048576) = %q, want %q", rate, expected)
	}
}

func TestSnapshot_String(t *testing.T) {
	snapshot := Snapshot{
		Timestamp:             time.Now(),
		PacketsSent:           100,
		PacketsReceived:       90,
		PacketsLost:           10,
		PacketsRetried:        5,
		BytesSent:             10240,
		BytesReceived:         9216,
		Errors:                2,
		Timeouts:              1,
		Resets:                0,
		SendRateBytesPerSec:   1024,
		RecvRateBytesPerSec:   1024,
		SendRatePacketsPerSec: 10,
		RecvRatePacketsPerSec: 9,
		MinLatencyUs:          100,
		MaxLatencyUs:          500,
		AvgLatencyUs:          250,
		PacketLossRate:        10.0,
		Uptime:                time.Minute,
		TimeSinceReset:        30 * time.Second,
	}

	str := snapshot.String()
	if len(str) == 0 {
		t.Error("Snapshot.String() returned empty string")
	}

	// Проверяем, что в строке есть ключевые данные
	if !contains(str, "100") || !contains(str, "10.00%") {
		t.Error("Snapshot.String() missing expected data")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsAt(s, substr, 0))
}

func containsAt(s, substr string, start int) bool {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func BenchmarkStatistics_RecordPacketSent(b *testing.B) {
	stats := NewStatistics()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stats.RecordPacketSent(1000)
	}
}

func BenchmarkStatistics_RecordPacketReceived(b *testing.B) {
	stats := NewStatistics()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stats.RecordPacketReceived(1000)
	}
}

func BenchmarkStatistics_RecordLatency(b *testing.B) {
	stats := NewStatistics()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stats.RecordLatency(100)
	}
}

func BenchmarkStatistics_GetSnapshot(b *testing.B) {
	stats := NewStatistics()

	// Заполняем некоторыми данными
	for i := 0; i < 100; i++ {
		stats.RecordPacketSent(1000)
		stats.RecordPacketReceived(1000)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stats.GetSnapshot()
	}
}

func BenchmarkStatistics_ConcurrentReads(b *testing.B) {
	stats := NewStatistics()

	// Заполняем данными
	for i := 0; i < 1000; i++ {
		stats.RecordPacketSent(1000)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			stats.GetPacketsSent()
			stats.GetBytesSent()
			stats.GetSendRate()
		}
	})
}
