package tcpconn_test

import (
	"fmt"
	"log"
	"tcpconn"
	"time"
)

// Пример базового использования статистики
func ExampleStatistics_basic() {
	stats := tcpconn.NewStatistics()

	// Записываем отправленные пакеты
	stats.RecordPacketSent(1024)
	stats.RecordPacketSent(2048)
	stats.RecordPacketSent(512)

	// Записываем полученные пакеты
	stats.RecordPacketReceived(1024)
	stats.RecordPacketReceived(2048)

	// Записываем потерянный пакет
	stats.RecordPacketLost()

	// Получаем статистику
	fmt.Printf("Отправлено пакетов: %d\n", stats.GetPacketsSent())
	fmt.Printf("Получено пакетов: %d\n", stats.GetPacketsReceived())
	fmt.Printf("Потеряно пакетов: %d\n", stats.GetPacketsLost())
	fmt.Printf("Отправлено байт: %d\n", stats.GetBytesSent())
	fmt.Printf("Процент потерь: %.2f%%\n", stats.GetPacketLossRate())

	// Output:
	// Отправлено пакетов: 3
	// Получено пакетов: 2
	// Потеряно пакетов: 1
	// Отправлено байт: 3584
	// Процент потерь: 33.33%
}

// Пример отслеживания задержек
func ExampleStatistics_latency() {
	stats := tcpconn.NewStatistics()

	// Записываем несколько измерений задержки (в микросекундах)
	stats.RecordLatency(100)
	stats.RecordLatency(200)
	stats.RecordLatency(150)
	stats.RecordLatency(300)
	stats.RecordLatency(120)

	fmt.Printf("Минимальная задержка: %d μs\n", stats.GetMinLatency())
	fmt.Printf("Средняя задержка: %d μs\n", stats.GetAvgLatency())
	fmt.Printf("Максимальная задержка: %d μs\n", stats.GetMaxLatency())

	// Output:
	// Минимальная задержка: 100 μs
	// Средняя задержка: 174 μs
	// Максимальная задержка: 300 μs
}

// Пример использования снимка статистики
func ExampleStatistics_GetSnapshot() {
	stats := tcpconn.NewStatistics()

	// Симулируем передачу данных
	for i := 0; i < 100; i++ {
		stats.RecordPacketSent(1024)
	}

	for i := 0; i < 95; i++ {
		stats.RecordPacketReceived(1024)
	}

	stats.RecordPacketLost()
	stats.RecordLatency(150)

	// Получаем снимок
	snapshot := stats.GetSnapshot()

	fmt.Printf("Пакетов отправлено: %d\n", snapshot.PacketsSent)
	fmt.Printf("Пакетов получено: %d\n", snapshot.PacketsReceived)
	fmt.Printf("Процент потерь: %.2f%%\n", snapshot.PacketLossRate)

	// Output:
	// Пакетов отправлено: 100
	// Пакетов получено: 95
	// Процент потерь: 1.00%
}

// Пример форматирования байтов
func ExampleFormatBytes() {
	fmt.Println(tcpconn.FormatBytes(0))
	fmt.Println(tcpconn.FormatBytes(512))
	fmt.Println(tcpconn.FormatBytes(1024))
	fmt.Println(tcpconn.FormatBytes(1536))
	fmt.Println(tcpconn.FormatBytes(1024 * 1024))
	fmt.Println(tcpconn.FormatBytes(1024 * 1024 * 1024))

	// Output:
	// 0 B
	// 512 B
	// 1.00 KiB
	// 1.50 KiB
	// 1.00 MiB
	// 1.00 GiB
}

// Пример форматирования скорости передачи
func ExampleFormatRate() {
	fmt.Println(tcpconn.FormatRate(1024))             // 1 KiB/s
	fmt.Println(tcpconn.FormatRate(1024 * 1024))      // 1 MiB/s
	fmt.Println(tcpconn.FormatRate(10 * 1024 * 1024)) // 10 MiB/s

	// Output:
	// 1.00 KiB/s
	// 1.00 MiB/s
	// 10.00 MiB/s
}

// Пример сброса статистики
func ExampleStatistics_Reset() {
	stats := tcpconn.NewStatistics()

	// Записываем данные
	stats.RecordPacketSent(1024)
	stats.RecordPacketSent(2048)
	fmt.Printf("До сброса: %d пакетов\n", stats.GetPacketsSent())

	// Сбрасываем статистику
	stats.Reset()
	fmt.Printf("После сброса: %d пакетов\n", stats.GetPacketsSent())

	// Output:
	// До сброса: 2 пакетов
	// После сброса: 0 пакетов
}

// Пример использования с TCPConnection
func ExampleTCPConnection_statistics() {
	conn, _ := tcpconn.NewTCPConnection(4096)
	conn.Connect()

	// Отправляем данные
	conn.Write([]byte("Hello, World!"))
	conn.Write([]byte("Test message"))

	// Получаем статистику
	snapshot := conn.GetStatisticsSnapshot()
	fmt.Printf("Отправлено пакетов: %d\n", snapshot.PacketsSent)

	// Output:
	// Отправлено пакетов: 2
}

// Пример мониторинга в реальном времени
func ExampleStatistics_monitoring() {
	stats := tcpconn.NewStatistics()

	// Симулируем передачу данных
	go func() {
		for i := 0; i < 5; i++ {
			stats.RecordPacketSent(1024)
			time.Sleep(10 * time.Millisecond)
		}
	}()

	// Ждём накопления данных
	time.Sleep(100 * time.Millisecond)

	// Получаем текущие показатели
	snapshot := stats.GetSnapshot()
	fmt.Printf("Отправлено: %d пакетов\n", snapshot.PacketsSent)
	fmt.Printf("Uptime: больше 0: %v\n", snapshot.Uptime > 0)

	// Output:
	// Отправлено: 5 пакетов
	// Uptime: больше 0: true
}

// Пример отслеживания ошибок
func ExampleStatistics_errors() {
	stats := tcpconn.NewStatistics()

	// Записываем различные типы ошибок
	stats.RecordError()
	stats.RecordError()
	stats.RecordTimeout()
	stats.RecordReset()

	fmt.Printf("Ошибок: %d\n", stats.GetErrors())
	fmt.Printf("Таймаутов: %d\n", stats.GetTimeouts())
	fmt.Printf("Сбросов: %d\n", stats.GetResets())

	// Output:
	// Ошибок: 2
	// Таймаутов: 1
	// Сбросов: 1
}

// Пример расчета процента успешности
func ExampleStatistics_successRate() {
	stats := tcpconn.NewStatistics()

	// Отправляем 100 пакетов
	for i := 0; i < 100; i++ {
		stats.RecordPacketSent(1024)
	}

	// 5 пакетов потеряны
	for i := 0; i < 5; i++ {
		stats.RecordPacketLost()
	}

	lossRate := stats.GetPacketLossRate()
	successRate := 100.0 - lossRate

	fmt.Printf("Процент потерь: %.1f%%\n", lossRate)
	fmt.Printf("Процент успешности: %.1f%%\n", successRate)

	// Output:
	// Процент потерь: 5.0%
	// Процент успешности: 95.0%
}

// Пример логирования статистики
func ExampleStatistics_logging() {
	stats := tcpconn.NewStatistics()

	// Записываем данные
	stats.RecordPacketSent(1024)
	stats.RecordPacketReceived(512)
	stats.RecordLatency(150)

	// Получаем полный снимок и выводим в лог
	snapshot := stats.GetSnapshot()

	log.Printf("=== Статистика соединения ===")
	log.Printf("Отправлено: %d пакетов (%s)",
		snapshot.PacketsSent,
		tcpconn.FormatBytes(snapshot.BytesSent))
	log.Printf("Получено: %d пакетов (%s)",
		snapshot.PacketsReceived,
		tcpconn.FormatBytes(snapshot.BytesReceived))
	log.Printf("Задержка: min=%dμs avg=%dμs max=%dμs",
		snapshot.MinLatencyUs,
		snapshot.AvgLatencyUs,
		snapshot.MaxLatencyUs)

	fmt.Println("Логирование выполнено")

	// Output:
	// Логирование выполнено
}

// Пример периодического сбора метрик
func ExampleStatistics_periodicCollection() {
	stats := tcpconn.NewStatistics()

	// Симулируем работу
	for i := 0; i < 10; i++ {
		stats.RecordPacketSent(1024)
		stats.RecordPacketReceived(1024)
	}

	// Собираем метрики каждую секунду (симуляция)
	snapshot := stats.GetSnapshot()

	fmt.Printf("Собрано метрик в момент времени %v\n", snapshot.Timestamp.Unix() > 0)
	fmt.Printf("Пакетов отправлено: %d\n", snapshot.PacketsSent)
	fmt.Printf("Пакетов получено: %d\n", snapshot.PacketsReceived)

	// Output:
	// Собрано метрик в момент времени true
	// Пакетов отправлено: 10
	// Пакетов получено: 10
}
