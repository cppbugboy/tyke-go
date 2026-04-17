#!/usr/bin/env python3
"""
Tyke Go Build Script

This script provides automated build functionality for the Tyke Go library,
supporting debug/release modes, testing, and incremental builds.

Usage:
    python build.py [options]

Options:
    -h, --help      Show this help message and exit
    --debug         Build in debug mode (no optimization)
    --release       Build in release mode with optimization (default)
    --test          Run tests after build
    --bench         Run benchmarks
    --cover         Run tests with coverage
    --clean         Clean build artifacts
    --force         Force rebuild (ignore incremental build cache)
    --verbose       Enable verbose output
    --lint          Run linter (golangci-lint)
    --doc           Generate documentation

Examples:
    python build.py                        # Build in release mode
    python build.py --debug                # Build in debug mode
    python build.py --test                 # Build and run tests
    python build.py --test --cover         # Build and run tests with coverage
    python build.py --clean                # Clean all build artifacts
    python build.py --force                # Force rebuild
    python build.py --lint                 # Run linter

Build Output:
    - Binaries: cmd/server/server, cmd/client/client
    - Coverage: coverage.html (when using --cover)
"""

import os
import sys
import json
import hashlib
import subprocess
import argparse
import platform
import shutil
from pathlib import Path
from datetime import datetime
from typing import Dict, List, Optional, Tuple


class BuildConfig:
    """Build configuration class."""
    
    def __init__(self, project_dir: Path):
        self.project_dir = project_dir
        self.cache_dir = project_dir / ".build_cache"
        self.cache_file = self.cache_dir / "go_build_cache.json"
        
        self.system = platform.system().lower()
        self.is_windows = self.system == "windows"
        self.is_linux = self.system == "linux"
        
        self.cache_dir.mkdir(exist_ok=True)
    
    def get_output_dir(self, build_type: str) -> Path:
        return self.project_dir / f"bin-{build_type}"


class IncrementalBuilder:
    """Incremental build support with file hash caching."""
    
    def __init__(self, config: BuildConfig):
        self.config = config
        self.cache = self._load_cache()
    
    def _load_cache(self) -> Dict:
        if self.config.cache_file.exists():
            try:
                with open(self.config.cache_file, 'r', encoding='utf-8') as f:
                    return json.load(f)
            except (json.JSONDecodeError, IOError):
                return {}
        return {}
    
    def _save_cache(self):
        with open(self.config.cache_file, 'w', encoding='utf-8') as f:
            json.dump(self.cache, f, indent=2)
    
    def _compute_file_hash(self, file_path: Path) -> str:
        hasher = hashlib.md5()
        with open(file_path, 'rb') as f:
            for chunk in iter(lambda: f.read(8192), b''):
                hasher.update(chunk)
        return hasher.hexdigest()
    
    def _compute_source_hash(self) -> str:
        hasher = hashlib.md5()
        
        for file_path in sorted(self.config.project_dir.rglob('*.go')):
            if file_path.is_file():
                file_hash = self._compute_file_hash(file_path)
                relative_path = file_path.relative_to(self.config.project_dir)
                hasher.update(str(relative_path).encode())
                hasher.update(file_hash.encode())
        
        go_mod = self.config.project_dir / "go.mod"
        if go_mod.exists():
            mod_hash = self._compute_file_hash(go_mod)
            hasher.update(b'go.mod')
            hasher.update(mod_hash.encode())
        
        go_sum = self.config.project_dir / "go.sum"
        if go_sum.exists():
            sum_hash = self._compute_file_hash(go_sum)
            hasher.update(b'go.sum')
            hasher.update(sum_hash.encode())
        
        return hasher.hexdigest()
    
    def needs_rebuild(self, build_type: str) -> bool:
        cache_key = build_type
        current_hash = self._compute_source_hash()
        cached_hash = self.cache.get(cache_key, {}).get('hash', '')
        
        if current_hash != cached_hash:
            self.cache[cache_key] = {
                'hash': current_hash,
                'last_build': datetime.now().isoformat()
            }
            return True
        return False
    
    def mark_built(self, build_type: str):
        cache_key = build_type
        if cache_key not in self.cache:
            self.cache[cache_key] = {}
        self.cache[cache_key]['last_build'] = datetime.now().isoformat()
        self._save_cache()


class GoBuilder:
    """Go build implementation."""
    
    def __init__(self, config: BuildConfig, incremental: IncrementalBuilder):
        self.config = config
        self.incremental = incremental
    
    def build(self, build_type: str, verbose: bool = False) -> bool:
        print(f"[BUILD] Building Go project ({build_type})...")
        
        ldflags = []
        if build_type == 'release':
            ldflags = ['-ldflags', '-s -w']
        
        build_args = ['go', 'build']
        if verbose:
            build_args.append('-v')
        build_args.extend(ldflags)
        build_args.append('./...')
        
        env = os.environ.copy()
        if build_type == 'release':
            env['CGO_ENABLED'] = '0'
        
        try:
            result = subprocess.run(
                build_args,
                cwd=self.config.project_dir,
                env=env,
                capture_output=True,
                text=True
            )
            
            if result.returncode != 0:
                print(f"[ERROR] Build failed:")
                print(result.stderr)
                return False
            
            self.incremental.mark_built(build_type)
            print(f"[BUILD] Build successful!")
            return True
            
        except Exception as e:
            print(f"[ERROR] Build failed with exception: {e}")
            return False
    
    def test(self, verbose: bool = False, coverage: bool = False) -> bool:
        print("[TEST] Running tests...")
        
        test_args = ['go', 'test']
        if verbose:
            test_args.append('-v')
        
        if coverage:
            test_args.extend(['-coverprofile=coverage.out', '-covermode=atomic'])
        
        test_args.append('./...')
        
        try:
            result = subprocess.run(
                test_args,
                cwd=self.config.project_dir,
                capture_output=True,
                text=True
            )
            
            if result.returncode != 0:
                print(f"[ERROR] Tests failed:")
                print(result.stderr)
                return False
            
            print("[TEST] All tests passed!")
            
            if coverage:
                self._generate_coverage_report()
            
            return True
            
        except Exception as e:
            print(f"[ERROR] Tests failed with exception: {e}")
            return False
    
    def _generate_coverage_report(self):
        print("[COVERAGE] Generating coverage report...")
        
        try:
            subprocess.run(
                ['go', 'tool', 'cover', '-html=coverage.out', '-o', 'coverage.html'],
                cwd=self.config.project_dir,
                capture_output=True
            )
            print(f"[COVERAGE] Report generated: coverage.html")
        except Exception as e:
            print(f"[WARN] Failed to generate coverage report: {e}")
    
    def bench(self) -> bool:
        print("[BENCH] Running benchmarks...")
        
        try:
            result = subprocess.run(
                ['go', 'test', '-bench=.', '-benchmem', './...'],
                cwd=self.config.project_dir,
                capture_output=True,
                text=True
            )
            
            print(result.stdout)
            
            if result.returncode != 0:
                print(f"[ERROR] Benchmarks failed:")
                print(result.stderr)
                return False
            
            print("[BENCH] Benchmarks completed!")
            return True
            
        except Exception as e:
            print(f"[ERROR] Benchmarks failed with exception: {e}")
            return False
    
    def lint(self) -> bool:
        print("[LINT] Running linter...")
        
        try:
            result = subprocess.run(
                ['golangci-lint', 'run', './...'],
                cwd=self.config.project_dir,
                capture_output=True,
                text=True
            )
            
            if result.returncode != 0:
                print(f"[LINT] Issues found:")
                print(result.stdout)
                return False
            
            print("[LINT] No issues found!")
            return True
            
        except FileNotFoundError:
            print("[WARN] golangci-lint not found. Skipping lint.")
            return True
        except Exception as e:
            print(f"[WARN] Lint failed: {e}")
            return True
    
    def doc(self) -> bool:
        print("[DOC] Generating documentation...")
        
        try:
            result = subprocess.run(
                ['go', 'doc', '-all', './...'],
                cwd=self.config.project_dir,
                capture_output=True,
                text=True
            )
            
            docs_dir = self.config.project_dir / "docs"
            docs_dir.mkdir(exist_ok=True)
            
            doc_file = docs_dir / "api-reference.md"
            with open(doc_file, 'w', encoding='utf-8') as f:
                f.write("# Tyke Go API Reference\n\n")
                f.write("```go\n")
                f.write(result.stdout)
                f.write("\n```\n")
            
            print(f"[DOC] Documentation generated: {doc_file}")
            return True
            
        except Exception as e:
            print(f"[ERROR] Documentation generation failed: {e}")
            return False
    
    def clean(self):
        print("[CLEAN] Cleaning build artifacts...")
        
        for bin_dir in self.config.project_dir.glob("bin-*"):
            if bin_dir.is_dir():
                print(f"  Removing {bin_dir}")
                shutil.rmtree(bin_dir)
        
        cache_dir = self.config.project_dir / ".build_cache"
        if cache_dir.exists():
            shutil.rmtree(cache_dir)
            print("  Build cache cleared")
        
        coverage_file = self.config.project_dir / "coverage.out"
        if coverage_file.exists():
            coverage_file.unlink()
        
        coverage_html = self.config.project_dir / "coverage.html"
        if coverage_html.exists():
            coverage_html.unlink()
        
        print("[CLEAN] Clean complete!")


def check_dependencies() -> bool:
    """Check if required dependencies are available."""
    print("[CHECK] Checking dependencies...")
    
    all_ok = True
    
    try:
        result = subprocess.run(['go', 'version'], capture_output=True, text=True)
        version = result.stdout.strip() if result.stdout else 'unknown'
        print(f"  [OK] Go: {version}")
    except FileNotFoundError:
        print("  [FAIL] Go not found")
        all_ok = False
    
    go_mod = Path(__file__).parent / "go.mod"
    if go_mod.exists():
        print(f"  [OK] go.mod found")
    else:
        print("  [FAIL] go.mod not found")
        all_ok = False
    
    return all_ok


def main():
    parser = argparse.ArgumentParser(
        description='Tyke Go Build Script',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  python build.py                        Build in release mode
  python build.py --debug                Build in debug mode
  python build.py --test                 Build and run tests
  python build.py --test --cover         Build and run tests with coverage
  python build.py --clean                Clean all build artifacts
  python build.py --force                Force rebuild
  python build.py --lint                 Run linter
        """
    )
    
    parser.add_argument('--debug', action='store_true', help='Build in debug mode')
    parser.add_argument('--release', action='store_true', help='Build in release mode (default)')
    parser.add_argument('--test', action='store_true', help='Run tests after build')
    parser.add_argument('--bench', action='store_true', help='Run benchmarks')
    parser.add_argument('--cover', action='store_true', help='Run tests with coverage')
    parser.add_argument('--clean', action='store_true', help='Clean build artifacts')
    parser.add_argument('--force', action='store_true', help='Force rebuild')
    parser.add_argument('--verbose', action='store_true', help='Enable verbose output')
    parser.add_argument('--lint', action='store_true', help='Run linter')
    parser.add_argument('--doc', action='store_true', help='Generate documentation')
    
    args = parser.parse_args()
    
    project_dir = Path(__file__).parent
    config = BuildConfig(project_dir)
    incremental = IncrementalBuilder(config)
    builder = GoBuilder(config, incremental)
    
    if args.clean:
        builder.clean()
        return 0
    
    if not check_dependencies():
        print("\n[ERROR] Dependency check failed. Please install missing dependencies.")
        return 1
    
    build_type = 'debug' if args.debug else 'release'
    
    print(f"\n{'='*60}")
    print(f"Tyke Go Build")
    print(f"{'='*60}")
    print(f"Build type: {build_type}")
    print(f"Run tests: {'yes' if args.test else 'no'}")
    print(f"{'='*60}\n")
    
    success = True
    
    if args.force or incremental.needs_rebuild(build_type):
        if not builder.build(build_type, args.verbose):
            success = False
    else:
        print("[SKIP] No changes detected")
    
    if success and args.test:
        if not builder.test(args.verbose, args.cover):
            success = False
    
    if success and args.bench:
        if not builder.bench():
            success = False
    
    if args.lint:
        if not builder.lint():
            success = False
    
    if args.doc:
        if not builder.doc():
            success = False
    
    if success:
        print(f"\n{'='*60}")
        print("[SUCCESS] Build completed successfully!")
        print(f"{'='*60}")
        return 0
    else:
        print(f"\n{'='*60}")
        print("[FAILED] Build failed!")
        print(f"{'='*60}")
        return 1


if __name__ == '__main__':
    sys.exit(main())
