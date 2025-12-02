# Модуль статистики передачи данных

Полное руководство по использованию модуля статистики для отслеживания передачи данных, скорости, потерь пакетов и задержек.

## Обзор

Модуль `Statistics` предоставляет:
- ✅ Подсчёт отправленных/полученных пакетов и байтов
- ✅ Отслеживание потерянных и повторно отправленных пакетов
- ✅ Расчёт скорости передачи в реальном времени
- ✅ Измерение задержек (min/avg/max)
- ✅ Подсчёт ошибок, таймаутов и сбросов
- ✅ Потокобезопасность (atomic operations + sync.RWMutex)
- ✅ Снимки статистики для мониторинга

## Быстрый старт

### Базовое использование

```go
package main

import (
    "fmt"
    "tcpconn"
)

func main() {
    stats := tcpconn.NewStatistics()
    
    // Записываем отправленные пакеты
    stats.RecordPacketSent(1024)  // 1 KB
    stats.RecordPacketSent(2048)  // 2 KB
    
    // Записываем полученные пакеты
    stats.RecordPacketReceived(1024)
    
    // Записываем потерянный пакет
    stats.RecordPacketLost()
    
    // Получаем статистику
    fmt.Printf("Отправлено: %d пакетов, %d байт\n", 
        stats.GetPacketsSent(), stats.GetBytesSent())
    fmt.Printf("Процент потерь: %.2f%%\n", 
        stats.GetPacketLossRate())
}
```

## API Reference

### Создание и управление

#### NewStatistics() *Statistics
Создаёт новый экземпляр статистики.

```go
stats := tcpconn.NewStatistics()
```

#### Reset()
Сбрасывает всю статистику к начальным значениям.

```go
stats.Reset()
```

### Запись событий

#### RecordPacketSent(bytes uint64)
Записывает отправленный пакет.

```go
stats.RecordPacketSent(1024) // отправлен пакет размером 1024 байта
```

#### RecordPacketReceived(bytes uint64)
Записывает полученный пакет.

```go
stats.RecordPacketReceived(2048) // получен пакет размером 2048 байт
```

#### RecordPacketLost()
Записывает потерянный пакет.

```go
stats.RecordPacketLost()
```

#### RecordPacketRetried()
Записывает повторную отправку пакета.

```go
stats.RecordPacketRetried()
```

#### RecordLatency(latencyUs uint64)
Записывает задержку в микросекундах.

```go
start := time.Now()
// ... операция ...
latency := time.Since(start).Microseconds()
stats.RecordLatency(uint64(latency))
```

#### RecordError()
Записывает ошибку.

```go
stats.RecordError()
```

#### RecordTimeout()
Записывает таймаут.

```go
stats.RecordTimeout()
```

#### RecordReset()
Записывает сброс соединения.

```go
stats.RecordReset()
```

### Чтение статистики

#### Счётчики пакетов

```go
sent := stats.GetPacketsSent()         // uint64
received := stats.GetPacketsReceived() // uint64
lost := stats.GetPacketsLost()         // uint64
retried := stats.GetPacketsRetried()   // uint64
```

#### Счётчики байтов

```go
bytesSent := stats.GetBytesSent()         // uint64
bytesReceived := stats.GetBytesReceived() // uint64
```

#### Скорость передачи

```go
// Байты в секунду
sendRate := stats.GetSendRate()     // float64
recvRate := stats.GetRecvRate()     // float64

// Пакеты в секунду
sendRatePkt := stats.GetSendRatePackets() // float64
recvRatePkt := stats.GetRecvRatePackets() // float64
```

#### Задержки (микросекунды)

```go
min := stats.GetMinLatency() // uint64
avg := stats.GetAvgLatency() // uint64
max := stats.GetMaxLatency() // uint64
```

#### Ошибки

```go
errors := stats.GetErrors()     // uint64
timeouts := stats.GetTimeouts() // uint64
resets := stats.GetResets()     // uint64
```

#### Производные метрики

```go
// Процент потерянных пакетов (0.0 - 100.0)
lossRate := stats.GetPacketLossRate() // float64

// Время работы
uptime := stats.GetUptime()                 // time.Duration
timeSinceReset := stats.GetTimeSinceReset() // time.Duration
```

### Снимки статистики

#### GetSnapshot() Snapshot
Возвращает снимок всей статистики в определённый момент времени.

```go
snapshot := stats.GetSnapshot()

fmt.Printf("Пакетов отправлено: %d\n", snapshot.PacketsSent)
fmt.Printf("Пакетов получено: %d\n", snapshot.PacketsReceived)
fmt.Printf("Скорость отправки: %.2f байт/с\n", snapshot.SendRateBytesPerSec)
fmt.Printf("Процент потерь: %.2f%%\n", snapshot.PacketLossRate)
```

#### Snapshot.String() string
Форматированное строковое представление статистики.

```go
snapshot := stats.GetSnapshot()
fmt.Println(snapshot.String())
```

Вывод:
```
Statistics Snapshot:
  Uptime: 5m30s (since reset: 2m15s)

  Packets:
    Sent:     1000 (10.50 pkt/s)
    Received: 950 (9.98 pkt/s)
    Lost:     50 (5.00%)
    Retried:  25

  Bytes:
    Sent:     1.00 MiB (200.00 KiB/s)
    Received: 975.00 KiB (195.00 KiB/s)

  Errors:
    Errors:   5
    Timeouts: 2
    Resets:   0

  Latency:
    Min: 100 μs
    Avg: 250 μs
    Max: 500 μs
```

## Утилиты форматирования

### FormatBytes(bytes uint64) string
Форматирует байты в читаемый вид.

```go
fmt.Println(tcpconn.FormatBytes(0))               // "0 B"
fmt.Println(tcpconn.FormatBytes(1024))            // "1.00 KiB"
fmt.Println(tcpconn.FormatBytes(1024 * 1024))     // "1.00 MiB"
fmt.Println(tcpconn.FormatBytes(1024 * 1024 * 1024)) // "1.00 GiB"
```

### FormatRate(bytesPerSec float64) string
Форматирует скорость передачи.

```go
fmt.Println(tcpconn.FormatRate(1024))         // "1.00 KiB/s"
fmt.Println(tcpconn.FormatRate(1024 * 1024))  // "1.00 MiB/s"
```

## Интеграция с TCPConnection

`TCPConnection` автоматически собирает статистику при каждой операции:

```go
conn, _ := tcpconn.NewTCPConnection(4096)
conn.Connect()

// Отправка автоматически записывается в статистику
conn.Write([]byte("Hello"))
conn.Write([]byte("World"))

// Получение статистики (всегда используйте Snapshot)
snapshot := conn.GetStatisticsSnapshot()
fmt.Printf("Отправлено пакетов: %d\n", snapshot.PacketsSent)
fmt.Printf("Байт отправлено: %d\n", snapshot.BytesSent)

// Сброс статистики
conn.ResetStatistics()
```

### Использование внешнего объекта Statistics

Если нужен прямой доступ к объекту статистики (например, для разделения между соединениями):

```go
// Создаём общий объект статистики
stats := tcpconn.NewStatistics()

// Создаём соединения с общей статистикой
conn1, _ := tcpconn.NewTCPConnectionWithStats(4096, stats)
conn2, _ := tcpconn.NewTCPConnectionWithStats(4096, stats)

// Оба соединения пишут в одну статистику
conn1.Write([]byte("data1"))
conn2.Write([]byte("data2"))

// Получаем общую статистику
fmt.Printf("Всего отправлено: %d пакетов\n", stats.GetPacketsSent())

// Или через snapshot
snapshot := stats.GetSnapshot()
fmt.Printf("Общая скорость: %s\n", tcpconn.FormatRate(snapshot.SendRateBytesPerSec))
```

## Примеры использования

### Пример 1: Мониторинг в реальном времени

```go
package main

import (
    "fmt"
    "tcpconn"
    "time"
)

func main() {
    stats := tcpconn.NewStatistics()
    
    // Запускаем мониторинг
    go func() {
        ticker := time.NewTicker(5 * time.Second)
        defer ticker.Stop()
        
        for range ticker.C {
            snapshot := stats.GetSnapshot()
            fmt.Printf("\n=== Статистика (каждые 5 сек) ===\n")
            fmt.Printf("Отправлено: %s (%s)\n",
                tcpconn.FormatBytes(snapshot.BytesSent),
                tcpconn.FormatRate(snapshot.SendRateBytesPerSec))
            fmt.Printf("Получено: %s (%s)\n",
                tcpconn.FormatBytes(snapshot.BytesReceived),
                tcpconn.FormatRate(snapshot.RecvRateBytesPerSec))
            fmt.Printf("Потери: %.2f%%\n", snapshot.PacketLossRate)
        }
    }()
    
    // Симулируем передачу данных
    for i := 0; i < 1000; i++ {
        stats.RecordPacketSent(1024)
        time.Sleep(10 * time.Millisecond)
    }
}
```

### Пример 2: Измерение задержки RTT

```go
func measureRTT(conn *tcpconn.TCPConnection) {
    // Создаём отдельный объект для измерения RTT
    rttStats := tcpconn.NewStatistics()
    
    for i := 0; i < 10; i++ {
        start := time.Now()
        
        // Отправка запроса
        conn.Write([]byte("PING"))
        
        // Получение ответа
        buf := make([]byte, 4)
        conn.Read(buf)
        
        // Записываем задержку
        rtt := time.Since(start).Microseconds()
        rttStats.RecordLatency(uint64(rtt))
        
        time.Sleep(100 * time.Millisecond)
    }
    
    // Получаем статистику RTT
    fmt.Printf("RTT: min=%dμs avg=%dμs max=%dμs\n",
        rttStats.GetMinLatency(),
        rttStats.GetAvgLatency(),
        rttStats.GetMaxLatency())
    
    // Общая статистика соединения
    snapshot := conn.GetStatisticsSnapshot()
    fmt.Printf("Всего передано: %d пакетов\n", snapshot.PacketsSent)
}
```

### Пример 3: Обнаружение потерь пакетов

```go
func detectPacketLoss(stats *tcpconn.Statistics) {
    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        lossRate := stats.GetPacketLossRate()
        
        if lossRate > 5.0 {
            fmt.Printf("⚠️  ПРЕДУПРЕЖДЕНИЕ: Высокий процент потерь %.2f%%\n", lossRate)
        } else if lossRate > 1.0 {
            fmt.Printf("⚡ Умеренные потери: %.2f%%\n", lossRate)
        } else {
            fmt.Printf("✅ Потери в норме: %.2f%%\n", lossRate)
        }
    }
}
```

### Пример 4: Экспорт метрик в Prometheus

```go
import (
    "github.com/prometheus/client_golang/prometheus"
    "tcpconn"
)

type StatsCollector struct {
    stats *tcpconn.Statistics
    
    packetsSent     prometheus.Counter
    packetsReceived prometheus.Counter
    packetsLost     prometheus.Counter
    sendRate        prometheus.Gauge
    recvRate        prometheus.Gauge
}

func NewStatsCollector(stats *tcpconn.Statistics) *StatsCollector {
    return &StatsCollector{
        stats: stats,
        packetsSent: prometheus.NewCounter(prometheus.CounterOpts{
            Name: "tcp_packets_sent_total",
            Help: "Total packets sent",
        }),
        // ... остальные метрики
    }
}

func (c *StatsCollector) Update() {
    snapshot := c.stats.GetSnapshot()
    
    c.packetsSent.Add(float64(snapshot.PacketsSent))
    c.packetsReceived.Add(float64(snapshot.PacketsReceived))
    c.sendRate.Set(snapshot.SendRateBytesPerSec)
    c.recvRate.Set(snapshot.RecvRateBytesPerSec)
}
```

### Пример 5: Логирование с уровнями

```go
import "log"

func logStatistics(stats *tcpconn.Statistics) {
    snapshot := stats.GetSnapshot()
    
    // INFO уровень - общая информация
    log.Printf("[INFO] Передано: %s отправлено, %s получено",
        tcpconn.FormatBytes(snapshot.BytesSent),
        tcpconn.FormatBytes(snapshot.BytesReceived))
    
    // WARNING - умеренные проблемы
    if snapshot.PacketLossRate > 1.0 {
        log.Printf("[WARN] Потери пакетов: %.2f%% (%d из %d)",
            snapshot.PacketLossRate,
            snapshot.PacketsLost,
            snapshot.PacketsSent)
    }
    
    // ERROR - критические проблемы
    if snapshot.PacketLossRate > 10.0 {
        log.Printf("[ERROR] Критические потери: %.2f%%", snapshot.PacketLossRate)
    }
    
    if snapshot.Errors > 0 {
        log.Printf("[ERROR] Ошибок: %d, Таймаутов: %d, Сбросов: %d",
            snapshot.Errors,
            snapshot.Timeouts,
            snapshot.Resets)
    }
    
    // DEBUG - детальная информация
    log.Printf("[DEBUG] Задержка: min=%dμs avg=%dμs max=%dμs",
        snapshot.MinLatencyUs,
        snapshot.AvgLatencyUs,
        snapshot.MaxLatencyUs)
}
```

### Пример 6: Оповещения по пороговым значениям

```go
type AlertConfig struct {
    MaxLossRate     float64
    MaxLatencyUs    uint64
    MaxErrors       uint64
    MinSendRate     float64 // минимальная скорость отправки
}

func checkAlerts(stats *tcpconn.Statistics, config AlertConfig) []string {
    snapshot := stats.GetSnapshot()
    var alerts []string
    
    if snapshot.PacketLossRate > config.MaxLossRate {
        alerts = append(alerts,
            fmt.Sprintf("Потери %.2f%% превышают порог %.2f%%",
                snapshot.PacketLossRate, config.MaxLossRate))
    }
    
    if snapshot.MaxLatencyUs > config.MaxLatencyUs {
        alerts = append(alerts,
            fmt.Sprintf("Задержка %dμs превышает порог %dμs",
                snapshot.MaxLatencyUs, config.MaxLatencyUs))
    }
    
    if snapshot.Errors > config.MaxErrors {
        alerts = append(alerts,
            fmt.Sprintf("Ошибок %d превышает порог %d",
                snapshot.Errors, config.MaxErrors))
    }
    
    if snapshot.SendRateBytesPerSec < config.MinSendRate {
        alerts = append(alerts,
            fmt.Sprintf("Скорость отправки %.2f ниже порога %.2f",
                snapshot.SendRateBytesPerSec, config.MinSendRate))
    }
    
    return alerts
}

// Использование
config := AlertConfig{
    MaxLossRate:  5.0,    // 5%
    MaxLatencyUs: 1000,   // 1ms
    MaxErrors:    10,
    MinSendRate:  100000, // 100 KB/s
}

alerts := checkAlerts(stats, config)
for _, alert := range alerts {
    log.Printf("⚠️  ALERT: %s", alert)
}
```

## Производительность

Бенчмарки на Apple M2:

```
BenchmarkStatistics_RecordPacketSent-8       ~50-100 ns/op  (0 allocs)
BenchmarkStatistics_RecordPacketReceived-8   ~50-100 ns/op  (0 allocs)
BenchmarkStatistics_RecordLatency-8          ~50-100 ns/op  (0 allocs)
BenchmarkStatistics_GetSnapshot-8            ~500 ns/op     (небольшие allocs)
BenchmarkStatistics_ConcurrentReads-8        ~20-30 ns/op   (0 allocs)
```

### Оптимизация производительности

1. **Используйте atomic операции** - все счётчики используют `atomic.AddUint64`
2. **Минимизируйте вызовы GetSnapshot** - делайте снимки периодически, не на каждую операцию
3. **История ограничена** - по умолчанию 60 точек данных для расчёта скорости
4. **Lock-free чтение** - большинство операций чтения не требуют блокировки

## Потокобезопасность

Все методы `Statistics` полностью потокобезопасны:
- Счётчики используют `atomic` операции
- Скорости и история защищены `sync.RWMutex`
- Безопасно использовать из множества горутин

```go
stats := tcpconn.NewStatistics()

// Безопасно из разных горутин
go func() {
    for {
        stats.RecordPacketSent(1024)
        time.Sleep(10 * time.Millisecond)
    }
}()

go func() {
    for {
        snapshot := stats.GetSnapshot()
        fmt.Println(snapshot.PacketsSent)
        time.Sleep(1 * time.Second)
    }
}()
```

## Структура Snapshot

```go
type Snapshot struct {
    Timestamp time.Time  // Время создания снимка
    
    // Счётчики пакетов
    PacketsSent     uint64
    PacketsReceived uint64
    PacketsLost     uint64
    PacketsRetried  uint64
    
    // Счётчики байтов
    BytesSent     uint64
    BytesReceived uint64
    
    // Ошибки
    Errors   uint64
    Timeouts uint64
    Resets   uint64
    
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
    PacketLossRate float64      // Процент потерь (0-100)
    Uptime         time.Duration // Время с создания
    TimeSinceReset time.Duration // Время с последнего сброса
}
```

## Best Practices

1. **Создавайте Statistics один раз** и переиспользуйте
2. **Записывайте события сразу** после их возникновения
3. **Используйте Reset()** для периодического сброса накопленных данных
4. **Делайте снимки периодически**, а не на каждую операцию
5. **Храните снимки** для построения графиков и анализа трендов
6. **Настройте оповещения** на критические пороговые значения
7. **Логируйте статистику** для последующего анализа
8. **Используйте GetStatisticsSnapshot()** вместо прямого доступа к Statistics
9. **Разделяйте статистику** через NewTCPConnectionWithStats() если нужен внешний доступ
10. **Не храните указатели** на внутренние объекты - используйте снимки

## Известные ограничения

- История ограничена 60 точками данных (можно изменить при необходимости)
- Скорость рассчитывается за последние 10 секунд
- Минимальная латентность инициализируется максимальным uint64

## Troubleshooting

**Q: Скорость всегда 0**  
A: Скорость рассчитывается по истории. Необходимо минимум 2 точки данных с интервалом во времени.

**Q: GetMinLatency() возвращает 0**  
A: Если задержки не записывались, минимум возвращает 0. Используйте RecordLatency().

**Q: Процент потерь некорректен**  
A: Убедитесь, что вы вызываете RecordPacketLost() для каждого потерянного пакета.

**Q: Статистика после Reset() не обнулилась полностью**  
A: Проверьте, что нет других горутин, которые продолжают записывать данные.

## См. также

- [README.md](README.md) - Общая документация
- [examples_test.go](examples_test.go) - Примеры использования
- [statistics_test.go](statistics_test.go) - Тесты
- [statistics_examples_test.go](statistics_examples_test.go) - Документированные примеры