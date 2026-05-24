package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"ric/dispatcher"
)

// Export builds a full backup image including all keys and stops the container.
func Export(inputs []string, flagArgs []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot get current directory: %w", err)
	}
	name := filepath.Base(cwd)

	checkCmd := exec.Command("docker", "inspect", name)
	if err := checkCmd.Run(); err != nil {
		return fmt.Errorf("container %s not found — are you inside a ric project directory?", name)
	}

	// Determine output directory (default: parent directory)
	outputDir := filepath.Dir(cwd)
	if len(inputs) > 0 {
		outputDir = inputs[0]
	}

	// 1. Run production migrations
	fmt.Println("Running production migrations...")
	migrateCmd := exec.Command(
		"docker", "exec",
		"-w", "/workspace/"+name,
		"-e", "RAILS_ENV=production",
		name,
		"bin/rails", "db:migrate",
	)
	migrateCmd.Stdout = os.Stdout
	migrateCmd.Stderr = os.Stderr
	if err := migrateCmd.Run(); err != nil {
		return fmt.Errorf("migrate failed: %w", err)
	}

	// 2. Seed production database
	fmt.Println("Seeding production database...")
	seedCmd := exec.Command(
		"docker", "exec",
		"-w", "/workspace/"+name,
		"-e", "RAILS_ENV=production",
		name,
		"bin/rails", "db:seed",
	)
	seedCmd.Stdout = os.Stdout
	seedCmd.Stderr = os.Stderr
	_ = seedCmd.Run()

	// 3. Precompile assets
	fmt.Println("Precompiling assets for production...")
	precompileCmd := exec.Command(
		"docker", "exec",
		"-w", "/workspace/"+name,
		"-e", "RAILS_ENV=production",
		name,
		"bin/rails", "assets:precompile",
	)
	precompileCmd.Stdout = os.Stdout
	precompileCmd.Stderr = os.Stderr
	if err := precompileCmd.Run(); err != nil {
		return fmt.Errorf("assets precompile failed: %w", err)
	}

	// 4. Stop the container
	fmt.Println("Stopping container...")
	stopCmd := exec.Command("docker", "stop", name)
	stopCmd.Stdout = os.Stdout
	stopCmd.Stderr = os.Stderr
	if err := stopCmd.Run(); err != nil {
		return fmt.Errorf("stop failed: %w", err)
	}

	// 5. Commit to a new image
	fmt.Println("Committing container to image...")
	commitCmd := exec.Command("docker", "commit", name, name+":latest")
	commitCmd.Stdout = os.Stdout
	commitCmd.Stderr = os.Stderr
	if err := commitCmd.Run(); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}

	// 6. Save to .tar
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

// prepareDeployImage creates a key‑free Docker image for deployment.
// It does NOT stop the local dev container.
func prepareDeployImage(name, outputDir string) error {
	projectPath := "/workspace/" + name

	// 1. Production migrations
	fmt.Println("Running production migrations...")
	migrateCmd := exec.Command("docker", "exec", "-w", projectPath,
		"-e", "RAILS_ENV=production", name, "bin/rails", "db:migrate")
	migrateCmd.Stdout = os.Stdout
	migrateCmd.Stderr = os.Stderr
	if err := migrateCmd.Run(); err != nil {
		return fmt.Errorf("migrate failed: %w", err)
	}

	// 2. Seed (non‑fatal)
	fmt.Println("Seeding production database...")
	seedCmd := exec.Command("docker", "exec", "-w", projectPath,
		"-e", "RAILS_ENV=production", name, "bin/rails", "db:seed")
	seedCmd.Stdout = os.Stdout
	seedCmd.Stderr = os.Stderr
	_ = seedCmd.Run()

	// 3. Precompile assets
	fmt.Println("Precompiling assets for production...")
	precompileCmd := exec.Command("docker", "exec", "-w", projectPath,
		"-e", "RAILS_ENV=production", name, "bin/rails", "assets:precompile")
	precompileCmd.Stdout = os.Stdout
	precompileCmd.Stderr = os.Stderr
	if err := precompileCmd.Run(); err != nil {
		return fmt.Errorf("assets precompile failed: %w", err)
	}

	// 4. Remove all secret keys
	fmt.Println("Removing secret keys for deployment...")
	for _, key := range []string{"config/master.key", "config/credentials/production.key"} {
		rmCmd := exec.Command("docker", "exec", name, "rm", "-f", key)
		rmCmd.Stdout = os.Stdout
		rmCmd.Stderr = os.Stderr
		if err := rmCmd.Run(); err != nil {
			return fmt.Errorf("remove %s: %w", key, err)
		}
	}

	// 5. Commit running container to temporary tag
	deployTag := name + ":ric-deploy"
	fmt.Println("Committing deploy image...")
	commitCmd := exec.Command("docker", "commit", name, deployTag)
	commitCmd.Stdout = os.Stdout
	commitCmd.Stderr = os.Stderr
	if err := commitCmd.Run(); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}

	// 6. Save to .tar
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

	// 7. Clean up temporary image
	exec.Command("docker", "rmi", deployTag).Run()

	fmt.Printf("Deploy image ready: %s\n", tarFile)
	return nil
}

func init() {
	dispatcher.Register("export", Export)
}
