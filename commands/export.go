package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"ric/dispatcher"
)

// Export builds a full backup image (including keys) and stops the container.
func Export(inputs []string, flagArgs []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot get current directory: %w", err)
	}
	name := filepath.Base(cwd)

	if err := exec.Command("docker", "inspect", name).Run(); err != nil {
		return fmt.Errorf("container %s not found — are you inside a ric project directory?", name)
	}

	outputDir := filepath.Dir(cwd)
	if len(inputs) > 0 {
		outputDir = inputs[0]
	}

	// 1. Migrations
	fmt.Println("Running production migrations...")
	migrateCmd := exec.Command("docker", "exec", "-w", "/workspace/"+name,
		"-e", "RAILS_ENV=production", name, "bin/rails", "db:migrate")
	migrateCmd.Stdout = os.Stdout
	migrateCmd.Stderr = os.Stderr
	if err := migrateCmd.Run(); err != nil {
		return fmt.Errorf("migrate failed: %w", err)
	}

	// 2. Seed
	fmt.Println("Seeding production database...")
	seedCmd := exec.Command("docker", "exec", "-w", "/workspace/"+name,
		"-e", "RAILS_ENV=production", name, "bin/rails", "db:seed")
	seedCmd.Stdout = os.Stdout
	seedCmd.Stderr = os.Stderr
	_ = seedCmd.Run()

	// 3. Precompile
	fmt.Println("Precompiling assets for production...")
	precompileCmd := exec.Command("docker", "exec", "-w", "/workspace/"+name,
		"-e", "RAILS_ENV=production", name, "bin/rails", "assets:precompile")
	precompileCmd.Stdout = os.Stdout
	precompileCmd.Stderr = os.Stderr
	if err := precompileCmd.Run(); err != nil {
		return fmt.Errorf("assets precompile failed: %w", err)
	}

	// 4. Stop & commit
	fmt.Println("Stopping container...")
	stopCmd := exec.Command("docker", "stop", name)
	stopCmd.Stdout = os.Stdout
	stopCmd.Stderr = os.Stderr
	if err := stopCmd.Run(); err != nil {
		return fmt.Errorf("stop failed: %w", err)
	}

	fmt.Println("Committing container to image...")
	commitCmd := exec.Command("docker", "commit", name, name+":latest")
	commitCmd.Stdout = os.Stdout
	commitCmd.Stderr = os.Stderr
	if err := commitCmd.Run(); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}

	// 5. Save to .tar
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("cannot create output directory: %w", err)
	}
	tarFile := filepath.Join(outputDir, name+".tar")
	fmt.Printf("Exporting image to %s...\n", tarFile)
	saveCmd := exec.Command("docker", "save", "-o", tarFile, name+":latest")
	saveCmd.Stdout = os.Stdout
	saveCmd.Stderr = os.Stderr
	if err := saveCmd.Run(); err != nil {
		return fmt.Errorf("save failed: %w", err)
	}

	fmt.Printf("Export complete: %s\n", tarFile)
	return nil
}

// prepareDeployImage creates a key‑free and database‑free deployment image.
// It restores the dev container to its original state after commit.
func prepareDeployImage(name, outputDir string) error {
	projectPath := "/workspace/" + name

	// 1. Precompile assets
	fmt.Println("Precompiling assets for production...")
	precompileCmd := exec.Command("docker", "exec", "-w", projectPath,
		"-e", "RAILS_ENV=production", name, "bin/rails", "assets:precompile")
	precompileCmd.Stdout = os.Stdout
	precompileCmd.Stderr = os.Stderr
	if err := precompileCmd.Run(); err != nil {
		return fmt.Errorf("assets precompile failed: %w", err)
	}

	// 2. Remove dev key and dev database (temporarily)
	fmt.Println("Preparing image for deployment...")
	cleanCmd := exec.Command("docker", "exec", name, "sh", "-c",
		fmt.Sprintf("rm -f %s/config/master.key %s/storage/development.sqlite3*", projectPath, projectPath))
	cleanCmd.Stdout = os.Stdout
	cleanCmd.Stderr = os.Stderr
	if err := cleanCmd.Run(); err != nil {
		return fmt.Errorf("clean dev artifacts: %w", err)
	}

	// 3. Commit
	deployTag := name + ":ric-deploy"
	fmt.Println("Committing deploy image...")
	commitCmd := exec.Command("docker", "commit", name, deployTag)
	commitCmd.Stdout = os.Stdout
	commitCmd.Stderr = os.Stderr
	if err := commitCmd.Run(); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}

	// 4. Restore dev key and dev database from host
	fmt.Println("Restoring dev environment...")
	hostProject := filepath.Join(".", name) // current directory
	cpKey := exec.Command("docker", "cp", filepath.Join(hostProject, "config", "master.key"),
		fmt.Sprintf("%s:%s/config/master.key", name, projectPath))
	cpKey.Stdout = os.Stdout
	cpKey.Stderr = os.Stderr
	_ = cpKey.Run() // ignore if master.key doesn't exist on host yet

	cpDB := exec.Command("docker", "cp", filepath.Join(hostProject, "storage", "development.sqlite3"),
		fmt.Sprintf("%s:%s/storage/development.sqlite3", name, projectPath))
	cpDB.Stdout = os.Stdout
	cpDB.Stderr = os.Stderr
	_ = cpDB.Run() // ignore if dev database doesn't exist on host

	// 5. Save .tar
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	tarFile := filepath.Join(outputDir, name+".tar")
	fmt.Printf("Exporting deploy image to %s...\n", tarFile)
	saveCmd := exec.Command("docker", "save", "-o", tarFile, deployTag)
	saveCmd.Stdout = os.Stdout
	saveCmd.Stderr = os.Stderr
	if err := saveCmd.Run(); err != nil {
		return fmt.Errorf("save failed: %w", err)
	}

	// 6. Cleanup
	exec.Command("docker", "rmi", deployTag).Run()
	_ = exec.Command("docker", "exec", name, "sh", "-c",
		fmt.Sprintf("rm -f %s/storage/production*.sqlite3*", projectPath)).Run()
	exec.Command("docker", "exec", "-w", projectPath, name, "bin/rails", "assets:clobber").Run()

	fmt.Printf("Deploy image ready: %s\n", tarFile)
	return nil
}

func init() {
	dispatcher.Register("export", Export)
}
