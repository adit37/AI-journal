# AI-Powered Productivity Journal (Backend API) 🚀

Sebuah RESTful API Backend yang dibangun menggunakan **Go (Golang)** untuk mencatat aktivitas harian. Sistem ini terintegrasi langsung dengan **Google Gemini AI** untuk menganalisis produktivitas, membaca kondisi emosional (EQ), dan memberikan evaluasi serta motivasi taktis secara dinamis.

## 🛠️ Tech Stack & Architecture
* **Language:** Go (Golang) - Menggunakan Standard Library `net/http`
* **Database:** SQLite (In-file persistence database)
* **AI Integration:** Google Gemini 2.0 Flash API (Prompt Engineering for JSON output)
* **Containerization:** Docker Ready (Dockerfile included)

## 📌 Features
* `POST /journal`: Menyimpan aktivitas dan men-trigger AI untuk melakukan analisis (*scoring, EQ check, motivation*).
* `GET /journal`: Mengambil seluruh riwayat jurnal yang tersimpan permanen di dalam SQLite.
* `PUT /journal?id=<id>`: Memperbarui (edit) entri jurnal dan meminta analisis ulang AI Gemini.
* `DELETE /journal?id=<id>`: Menghapus riwayat jurnal.
* Auto-structuring response AI menggunakan teknik Nested JSON Unmarshaling.

## 🚀 How to Run Locally
1. Clone repositori ini.
2. Dapatkan Gemini API Key dari Google AI Studio.
3. Set environment variable di terminal: `export GEMINI_API_KEY="kunci-rahasiamu"` (Linux/Mac) atau `set GEMINI_API_KEY="kunci-rahasiamu"` (Windows).
4. Jalankan `go mod tidy` untuk mengunduh driver SQLite.
5. Jalankan `go run main.go`.