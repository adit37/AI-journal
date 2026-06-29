package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	_ "modernc.org/sqlite" // Import driver SQLite secara anonim (menggunakan garis bawah)
)

// Struktur data jurnal baru kita (Struct V2)
type journalEntry struct {
	ID          int       `json:"id"`          // ID unik untuk setiap entri jurnal
	Date        string    `json:"date"`        // Format: YYYY-MM-DD
	Activity    string    `json:"activity"`    // Judul aktivitas
	Description string    `json:"description"` // Deskripsi singkat tentang aktivitas
	Score       int       `json:"score"`       // Skor dari 1-100 untuk menilai aktivitas
	EQAnalysis  string    `json:"eq_analysis"` // Analisis emosional
	Motivation  string    `json:"motivation"`  // Motivasi terkait aktivitas
	Evaluation  string    `json:"evaluation"`  // Evaluasi aktivitas
	CreatedAt   time.Time `json:"timestamp"`   // Waktu pencatatan
}

// Struktur untuk menampung hasil parsing dari Gemini khusus untuk kebutuhan kita
type geminiResponse struct {
	Score      int    `json:"score"`
	EQAnalysis string `json:"eq_analysis"`
	Motivation string `json:"motivation"`
	Evaluation string `json:"evaluation"`
}

// Variabel Global untuk Koneksi Database
var db *sql.DB

// Fungsi untuk Inisialisasi Database SQLite
func initDB() error {
	var err error
	db, err = sql.Open("sqlite", "./journal.db")
	if err != nil {
		panic(fmt.Errorf("gagal membuka database: %v", err))
	}

	// Membuat Tabel SQL jika belum ada.
	// Kita jadikan 'date' sebagai PRIMARY KEY (Satu hari, satu jurnal)
	createTableQuery := `CREATE TABLE IF NOT EXISTS journal_entries (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		date text NOT NULL,
		activity text NOT NULL,
		description text NOT NULL,
		score integer NOT NULL,
		eq_analysis text NOT NULL,
		motivation text NOT NULL,
		evaluation text NOT NULL,
		created_at datetime NOT NULL
	);`

	_, err = db.Exec(createTableQuery)
	if err != nil {
		panic(fmt.Errorf("Gagal membuat tabel: %v", err))
	}
	fmt.Println("Database SQLite berhasil diinisialisasi dan tabel siap digunakan.")
	return nil
}

// FUNGSI BARU: Menembak Google Gemini API yang Asli
func callGeminiAPI(description string) (geminiResponse, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return geminiResponse{}, fmt.Errorf("GEMINI_API_KEY tidak ditemukan di environment")
	}

	// URL resmi Google Gemini 1.5 Flash
	// "https://generativelanguage.googleapis.com/v1beta/models/gemini-flash-latest:generateContent"
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-3.1-flash-lite:generateContent?key=%s", apiKey)

	// Buat instruksi (System Prompt) agar Gemini mengembalikan format JSON yang kaku
	instruction := fmt.Sprintf(`
		Kamu adalah Asisten Jurnal Pribadi AI yang empatik, ceria, dan suportif.
		Analisis entri jurnal harian berikut: "%s".
		Berikan output harus berupa JSON mentah murni tanpa markdown, tanpa teks pembuka, dan tanpa tanda petik tiga
		Format JSON harus persis seperti ini:
		{
			"score": 85,
			"eq_analysis": "<analisis kecerdasan emosional (EQ) dari jurnal, maksimal 2 kalimat>",
			"motivation": "<kata-kata motivasi yang hangat, ceria, dan menyemangati>",
			"evaluation": "<evaluasi singkat tentang produktivitas atau pelajaran hari ini>"
		}`, description)
	// Terkadang jika kita menaruh tanda kurung siku/kurung sudut di dalam format JSON,
	// AI malah akan mencetak ulang tanda kurungnya.
	// Memberikan contoh angka bulat sering kali membuat output JSON dari AI jauh lebih stabil

	// Rakit Payload/Body Request sesuai standar dokumentasi Google
	payload := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{
						"text": instruction,
					},
				},
			},
		},
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return geminiResponse{}, fmt.Errorf("Gagal membuat payload JSON: %v", err)
	}
	// Buat request HTTP POST ke Google Gemini API
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return geminiResponse{}, fmt.Errorf("Gagal membuat request: %v", err)
	}
	// PENTING: Tutup jalur koneksi setelah data selesai dibaca agar tidak terjadi memory leak
	defer resp.Body.Close()

	// --- LOGIKA DEBUGGING BARU (SINAR-X) ---

	// Baca seluruh isi balasan dari Google mentah-mentah
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return geminiResponse{}, fmt.Errorf("Gagal membaca respons dari Gemini: %v", err)
	}
	// Cek apakah status HTTP-nya BUKAN 200 OK
	if resp.StatusCode != http.StatusOK {
		// Jika Google mengembalikan error (misal 400 Bad Request / 403 Forbidden)
		// Kembalikan isi error asli dari Google agar kita bisa baca di terminal/Postman
		return geminiResponse{}, fmt.Errorf("Respons Gemini tidak OK: %s, Body: %s", resp.Status, string(bodyBytes))
	}
	// Jika statusnya 200 OK, mari kita log (cetak) isinya di terminal agar kamu bisa lihat
	//fmt.Printf("DEBUG: Respons mentah dari Gemini: %s\n", string(bodyBytes))

	// Struktur penangkap respons mentah dari skema Google Gemini
	var geminiRawResponse struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	// Pakai json.Unmarshal ke byte array karena datanya sudah kita baca jadi byte di atas
	if err := json.Unmarshal(bodyBytes, &geminiRawResponse); err != nil {
		return geminiResponse{}, fmt.Errorf("Gagal decode respons mentah Gemini: %v", err)
	}

	// Ambil teks JSON dalam bentuk string yang dihasilkan oleh Gemini
	if len(geminiRawResponse.Candidates) == 0 || len(geminiRawResponse.Candidates[0].Content.Parts) == 0 {
		return geminiResponse{}, fmt.Errorf("Respons Gemini tidak memiliki konten yang valid")
	}

	geminiJSONText := geminiRawResponse.Candidates[0].Content.Parts[0].Text

	// Logika Pembersih Markdown
	geminiJSONText = strings.ReplaceAll(geminiJSONText, "```json", "")
	geminiJSONText = strings.ReplaceAll(geminiJSONText, "```", "")
	geminiJSONText = strings.TrimSpace(geminiJSONText)

	// Decode teks string tadi ke dalam Struct Akhir kita
	var result geminiResponse
	if err := json.Unmarshal([]byte(geminiJSONText), &result); err != nil {
		return geminiResponse{}, fmt.Errorf("Gagal decode JSON dari Gemini: %v", err)
	}
	return result, nil
}

// HANDLER UTAMA
func journalHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// ALUR GET: Mengambil semua daftar jurnal
	if r.Method == http.MethodGet {
		rows, err := db.Query("SELECT id, date, activity, description, score, eq_analysis, motivation, evaluation, created_at FROM journal_entries")
		if err != nil {
			http.Error(w, fmt.Sprintf("Gagal mengambil data jurnal: %v", err), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var entries []journalEntry
		for rows.Next() {
			var entry journalEntry
			err := rows.Scan(&entry.ID, &entry.Date, &entry.Activity, &entry.Description, &entry.Score, &entry.EQAnalysis, &entry.Motivation, &entry.Evaluation, &entry.CreatedAt)
			if err != nil {
				http.Error(w, fmt.Sprintf("Gagal memproses data jurnal: %v", err), http.StatusInternalServerError)
				return
			}
			entries = append(entries, entry)
		}
		json.NewEncoder(w).Encode(entries)
		return
	}

	// ALUR POST: Menambahkan entri jurnal baru
	if r.Method == http.MethodPost {
		var input struct {
			Activity    string `json:"activity"`
			Description string `json:"description"`
		}
		err := json.NewDecoder(r.Body).Decode(&input)
		if err != nil {
			http.Error(w, fmt.Sprintf("Request payload tidak valid %v", err), http.StatusBadRequest)
			return
		}

		today := time.Now().Format("2006-01-02") // Format YYYY-MM-DD
		currentTime := time.Now()

		// Panggil Gemini API untuk mendapatkan analisis
		geminiResult, err := callGeminiAPI(input.Description)
		if err != nil {
			http.Error(w, fmt.Sprintf("Gagal memanggil Gemini API: %v", err), http.StatusInternalServerError)
			return
		}

		// Mengeksekusi query INSERT ke tabel SQLite
		insertSQL := `INSERT INTO journal_entries (date, activity, description, score, eq_analysis, motivation, evaluation, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
		_, err = db.Exec(insertSQL, today, input.Activity, input.Description, geminiResult.Score, geminiResult.EQAnalysis, geminiResult.Motivation, geminiResult.Evaluation, currentTime)
		if err != nil {
			http.Error(w, fmt.Sprintf("Gagal menyimpan entri jurnal: %v", err), http.StatusInternalServerError)
			return
		}

		// Bungkus semua ke struct lengkap
		newEntry := journalEntry{
			ID:          0, // ID akan diisi otomatis oleh SQLite
			Date:        today,
			Activity:    input.Activity,
			Description: input.Description,
			Score:       geminiResult.Score,
			EQAnalysis:  geminiResult.EQAnalysis,
			Motivation:  geminiResult.Motivation,
			Evaluation:  geminiResult.Evaluation,
			CreatedAt:   currentTime,
		}

		// Kembalikan response ke user
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(newEntry)
		return
	}

	// ALUR PUT: Mengupdate entri jurnal yang ada (Minta analisis Gemini ulang)
	if r.Method == http.MethodPut {
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "ID jurnal diperlukan", http.StatusBadRequest)
			return
		}

		var input struct {
			Activity    string `json:"activity"`
			Description string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, fmt.Sprintf("Request payload tidak valid %v", err), http.StatusBadRequest)
			return
		}

		// Panggil Gemini API ulang untuk mendapatkan analisis baru
		geminiResult, err := callGeminiAPI(input.Description)
		if err != nil {
			http.Error(w, fmt.Sprintf("Gagal memanggil Gemini API: %v", err), http.StatusInternalServerError)
			return
		}

		updateSQL := `UPDATE journal_entries SET activity=?, description=?, score=?, eq_analysis=?, motivation=?, evaluation=? WHERE id=?`
		_, err = db.Exec(updateSQL, input.Activity, input.Description, geminiResult.Score, geminiResult.EQAnalysis, geminiResult.Motivation, geminiResult.Evaluation, id)
		if err != nil {
			http.Error(w, fmt.Sprintf("Gagal mengupdate entri jurnal: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "sukses", "message": "Jurnal berhasil diupdate"})
		return
	}

	// ALUR DELETE: Menghapus entri jurnal
	if r.Method == http.MethodDelete {
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "ID jurnal diperlukan", http.StatusBadRequest)
			return
		}

		deleteSQL := `DELETE FROM journal_entries WHERE id=?`
		_, err := db.Exec(deleteSQL, id)
		if err != nil {
			http.Error(w, fmt.Sprintf("Gagal menghapus entri jurnal: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "sukses", "message": "Jurnal berhasil dihapus"})
		return
	}

	http.Error(w, "Method Tidak tersedia", http.StatusMethodNotAllowed)
}

func helloHandler() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Sajikan file index.html di root directory
		http.ServeFile(w, r, "index.html")
	})
}

func main() {
	helloHandler()
	initDB() // Inisialisasi database SQLite
	http.HandleFunc("/journal", journalHandler)
	fmt.Println("Journal AI Aktif. Server is running on http://localhost:8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Printf("Error starting server: %v", err)
	}
}
