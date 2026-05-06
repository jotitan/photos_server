package main

import (
	"encoding/json"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Kagami/go-face"
)

const (
	matchThreshold float32 = 0.6
	serverPort             = ":8080"

	// imagePattern filters target images by filename suffix before extension.
	// Only files matching this pattern will be processed.
	// e.g. "-1080" accepts "img_blabla-1080.jpg" but rejects "img_blabla-250.jpg"
	imagePattern = "-1080"
)

// FaceResult represents an identified face.
type FaceResult struct {
	ID        string `json:"id"`
	ImageFile string `json:"image_file"`
}

type knownFace struct {
	id         string
	name       string
	descriptor face.Descriptor
}

// service holds the pre-loaded known faces data and worker pool.
type service struct {
	knownFaces  []knownFace
	descriptors []face.Descriptor
	labels      []int32
	pool        *WorkerPool
	rootDir     string
	modelsDir   string
}

func main() {
	if len(os.Args) < 4 {
		log.Println("Usage: face-detector-job <models_dir> <known_faces_dir> <root_dir>")
		os.Exit(1)
	}

	modelsDir := os.Args[1]
	knownDir := os.Args[2]
	rootDir := os.Args[3]

	svc := &service{rootDir: rootDir, modelsDir: modelsDir}
	svc.knownFaces, svc.descriptors, svc.labels = svc.loadKnownFaces(knownDir)
	log.Printf("Loaded %d known face(s)\n", len(svc.knownFaces))

	svc.pool = NewWorkerPool(svc)
	svc.pool.Start()

	http.HandleFunc("/status", handleStatus)
	http.HandleFunc("/identify", svc.handleIdentify)

	log.Printf("Server listening on %s\n", serverPort)
	log.Fatal(http.ListenAndServe(serverPort, nil))
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// handleIdentify handles POST /identify?folder=<target_folder>
func (svc *service) handleIdentify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	folder := r.URL.Query().Get("folder")
	if folder == "" {
		http.Error(w, "Missing 'folder' query parameter", http.StatusBadRequest)
		return
	}

	// Resolve relative folder against root directory
	fullPath := filepath.Join(svc.rootDir, folder)

	// Validate folder exists
	info, err := os.Stat(fullPath)
	if err != nil || !info.IsDir() {
		http.Error(w, "Folder not found or not a directory", http.StatusBadRequest)
		return
	}

	start := time.Now()
	results, numImages := svc.identifyFacesPooled(fullPath)
	elapsed := time.Since(start)

	response := struct {
		Results     []FaceResult `json:"results"`
		NumImages   int          `json:"num_images"`
		TotalTime   string       `json:"total_time"`
		AvgPerImage string       `json:"avg_per_image"`
	}{
		Results:   results,
		NumImages: numImages,
		TotalTime: elapsed.String(),
	}
	if numImages > 0 {
		response.AvgPerImage = (elapsed / time.Duration(numImages)).String()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (svc *service) loadKnownFaces(folder string) ([]knownFace, []face.Descriptor, []int32) {
	rec, err := face.NewRecognizer(svc.modelsDir)
	if err != nil {
		log.Fatalf("Failed to initialize recognizer: %v\n"+
			"Make sure model files are in the '%s' directory.\n", err, svc.modelsDir)
	}
	defer rec.Close()

	var knownFaces []knownFace
	var descriptors []face.Descriptor
	var labels []int32

	entries, err := os.ReadDir(folder)
	if err != nil {
		log.Fatalf("Failed to read known faces directory: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		imgPath := filepath.Join(folder, entry.Name())
		faces, err := rec.RecognizeFile(imgPath)
		if err != nil {
			log.Printf("Warning: skipping %s: %v", entry.Name(), err)
			continue
		}
		if len(faces) == 0 {
			log.Printf("Warning: no face found in %s, skipping", entry.Name())
			continue
		}

		name := entry.Name()
		ext := filepath.Ext(name)
		base := name[:len(name)-len(ext)]

		// Expected format: name.id.ext (e.g. "john.42.jpg" -> name="john", id="42")
		parts := strings.SplitN(base, ".", 2)
		personName := parts[0]
		personID := ""
		if len(parts) == 2 {
			personID = parts[1]
		}

		knownFaces = append(knownFaces, knownFace{
			id:         personID,
			name:       personName,
			descriptor: faces[0].Descriptor,
		})
		descriptors = append(descriptors, faces[0].Descriptor)
		labels = append(labels, int32(len(knownFaces)-1))
	}

	if len(knownFaces) == 0 {
		log.Fatal("No known faces loaded")
	}

	return knownFaces, descriptors, labels
}

// imageJob represents a single image to process, with a channel to send results back.
type imageJob struct {
	imgPath  string
	fileName string
	resultCh chan<- FaceResult
	done     chan<- struct{}
}

// WorkerPool manages a pool of persistent face recognition workers.
type WorkerPool struct {
	svc  *service
	jobs chan imageJob
}

// NewWorkerPool creates a new worker pool for the given service.
func NewWorkerPool(svc *service) *WorkerPool {
	return &WorkerPool{
		svc:  svc,
		jobs: make(chan imageJob, 100),
	}
}

// Start launches one worker per CPU core.
func (wp *WorkerPool) Start() {
	numWorkers := runtime.NumCPU()
	for i := 0; i < numWorkers; i++ {
		go wp.runWorker(i)
	}
	log.Printf("Started %d worker(s)\n", numWorkers)
}

// Add sends an image job to the worker pool.
func (wp *WorkerPool) Add(job imageJob) {
	wp.jobs <- job
}

func (wp *WorkerPool) runWorker(id int) {
	rec, err := face.NewRecognizer(wp.svc.modelsDir)
	if err != nil {
		log.Printf("Worker %d: failed to create recognizer: %v", id, err)
		return
	}
	defer rec.Close()
	rec.SetSamples(wp.svc.descriptors, wp.svc.labels)

	for job := range wp.jobs {
		targetFaces, err := rec.RecognizeFile(job.imgPath)
		if err != nil {
			log.Printf("Worker %d: skipping %s: %v", id, job.fileName, err)
			job.done <- struct{}{}
			continue
		}

		for _, f := range targetFaces {
			classID := rec.ClassifyThreshold(f.Descriptor, matchThreshold)
			if classID >= 0 {
				// Remove imagePattern from filename before returning
				cleanName := strings.Replace(job.fileName, imagePattern, "", 1)
				job.resultCh <- FaceResult{
					ID:        wp.svc.knownFaces[classID].id,
					ImageFile: cleanName,
				}
			}
		}
		job.done <- struct{}{}
	}
}

func filterFiles(entries []os.DirEntry) []os.DirEntry {
	var files []os.DirEntry
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := filepath.Ext(name)
		base := name[:len(name)-len(ext)]
		if !strings.HasSuffix(base, imagePattern) {
			continue
		}
		files = append(files, entry)
	}
	return files
}

// identifyFacesPooled dispatches images to the persistent worker pool
// and collects results via a per-request channel.
func (svc *service) identifyFacesPooled(targetFolder string) ([]FaceResult, int) {
	entries, err := os.ReadDir(targetFolder)
	if err != nil {
		log.Printf("Failed to read target folder: %v", err)
		return nil, 0
	}

	files := filterFiles(entries)
	if len(files) == 0 {
		return nil, 0
	}

	resultCh := make(chan FaceResult, 100)
	doneCh := make(chan struct{}, len(files))

	// Collect results concurrently to avoid deadlock
	var results []FaceResult
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for r := range resultCh {
			results = append(results, r)
		}
	}()

	// Dispatch all image jobs to the worker pool
	for _, entry := range files {
		svc.pool.Add(imageJob{
			imgPath:  filepath.Join(targetFolder, entry.Name()),
			fileName: entry.Name(),
			resultCh: resultCh,
			done:     doneCh,
		})
	}

	// Wait for all images to be processed by counting done signals
	for i := 0; i < len(files); i++ {
		<-doneCh
	}
	close(resultCh)
	wg.Wait()

	return results, len(files)
}
