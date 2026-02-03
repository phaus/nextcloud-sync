# Security Specification

## Overview

This document outlines the security requirements and implementations for the Nextcloud Sync CLI tool to ensure safe handling of credentials, data, and network communications.

## Threat Model

### Potential Threats
1. **Credential Exposure** - App passwords stored insecurely
2. **Man-in-the-Middle Attacks** - Network traffic interception
3. **Data Integrity** - File corruption during transfer
4. **Unauthorized Access** - Access to user files and credentials
5. **Configuration Tampering** - Malicious config file modifications

### Security Boundaries
- Local file system access
- Network communication with Nextcloud
- Configuration file storage
- Runtime memory management

## Credential Security

### App Password Storage

#### Encryption Requirements
- **Algorithm**: AES-256-GCM for authenticated encryption
- **Key Derivation**: PBKDF2 with 100,000 iterations
- **Salt**: Random 32-byte salt per password
- **Storage**: Encrypted blob in JSON config file

#### Encryption Implementation
```go
// Key derivation from machine-specific secret
key = PBKDF2(machineID + userHomeDir, salt, 100000)

// Authenticated encryption
encrypted = AES-256-GCM(plaintext, key, nonce)
```

#### Configuration File Security
- **Location**: `~/.nextcloud-sync/config.json`
- **Permissions**: 600 (read/write by owner only)
- **Backup**: Encrypted backup created on changes
- **Validation**: JSON schema validation on load

### Password Management Lifecycle

#### Generation
1. User generates app password in Nextcloud UI
2. Password entered once during CLI setup
3. Immediate encryption and storage
3. No plaintext password retention in memory

#### Storage
```
{
  "servers": {
    "default": {
      "url": "https://cloud.example.com",
      "username": "user@example.com",
      "app_password": {
        "encrypted": "base64_encrypted_blob",
        "salt": "base64_salt",
        "nonce": "base64_nonce",
        "algorithm": "aes-256-gcm"
      }
    }
  }
}
```

#### Rotation
- Support for password rotation
- Automatic invalidation of failed passwords
- Secure deletion of old encrypted credentials

## Network Security

### HTTPS Requirements
- **TLS Version**: Minimum TLS 1.2, preferably TLS 1.3
- **Certificate Validation**: Strict certificate chain verification
- **Hostname Verification**: Prevent man-in-the-middle attacks
- **HSTS Support**: Respect HTTP Strict Transport Security

### HTTP Client Security
```go
client := &http.Client{
    Timeout: 30 * time.Second,
    Transport: &http.Transport{
        TLSClientConfig: &tls.Config{
            MinVersion: tls.VersionTLS12,
            InsecureSkipVerify: false,
        },
    },
}
```

### Authentication Headers
- **Basic Auth**: Base64 encoding of `username:app_password`
- **Memory Management**: Zero-out authentication headers after use
- **Request Logging**: Never log authentication headers

### Network Timeout and Retry
- **Connection Timeout**: 30 seconds
- **Read Timeout**: 60 seconds for large files
- **Retry Logic**: Exponential backoff with jitter
- **Max Retries**: 3 attempts per operation

## File System Security

### Configuration File Protection
- **Permissions**: Strict 600 permissions on creation
- **Ownership**: Verify file ownership matches current user
- **Integrity**: JSON schema validation and checksum verification
- **Backup**: Encrypted backup before modifications

### Temporary File Security
- **Location**: System temp directory with secure permissions
- **Naming**: Random filenames to prevent guessing
- **Cleanup**: Immediate deletion after use
- **Permissions**: 600 permissions for temp files

### Synchronization Security
- **Path Validation**: Prevent directory traversal attacks
- **Symlink Handling**: Safe symlink resolution and following
- **File Permissions**: Preserve original permissions where possible
- **Ownership**: Maintain file ownership information

## Data Protection

### In-Transit Security
- **Encryption**: TLS for all network communications
- **Integrity**: Built-in TLS message authentication
- **Compression**: Optional compression with security considerations
- **Chunking**: Secure chunked uploads for large files

### At-Rest Security
- **Local Files**: No modification of existing file security
- **Remote Files**: Rely on Nextcloud's security model
- **Metadata**: Secure handling of file metadata
- **Logs**: No sensitive information in log files

## Input Validation

### URL Validation
- **Protocol**: Restrict to HTTPS only
- **Hostname**: DNS name validation and certificate matching
- **Path**: Directory traversal prevention
- **Query Parameters**: Sanitization of Nextcloud-specific parameters

### File Path Validation
```go
func validatePath(path string) error {
    // Clean path to resolve ".." and "."
    clean := filepath.Clean(path)
    
    // Ensure path doesn't escape allowed directories
    if strings.Contains(clean, "..") {
        return errors.New("path traversal detected")
    }
    
    // Additional platform-specific validations
    return nil
}
```

### Configuration Validation
- **JSON Schema**: Strict schema validation
- **Type Checking**: Runtime type verification
- **Range Validation**: Validate numeric ranges
- **Format Validation**: URL format, email format, etc.

## Error Handling Security

### Information Disclosure Prevention
- **Error Messages**: No sensitive information in error messages
- **Stack Traces**: Internal errors only in debug mode
- **Logging**: Sanitize log entries to remove credentials
- **User Output**: Safe error reporting for end users

### Attack Surface Reduction
- **Input Sanitization**: All external inputs validated
- **Command Injection**: No shell command construction from user input
- **File Operations**: Safe file path handling
- **Network Operations**: Restricted to necessary protocols only

## Secure Development Practices

### Dependency Management
- **Standard Library**: Prefer Go standard library for security
- **Vulnerability Scanning**: Regular security audits of dependencies
- **Minimal Dependencies**: Reduce attack surface through minimal deps
- **Security Updates**: Prompt security patch application

### Code Security
- **Memory Safety**: Zero out sensitive data after use
- **Information Flow**: Prevent sensitive data leakage
- **Input Validation**: Comprehensive input sanitization
- **Error Handling**: Secure error management

### Testing Security
- **Security Tests**: Unit tests for security-critical components
- **Penetration Testing**: Security assessment of authentication
- **Fuzz Testing**: Input validation robustness testing
- **Static Analysis**: Security-focused code analysis

## Compliance and Auditing

### Security Audit Trail
- **Access Logs**: File access and modification logging
- **Sync Operations**: Detailed operation logging
- **Authentication**: Failed authentication attempt logging
- **Configuration**: Configuration change logging

### Privacy Protection
- **Data Minimization**: Only collect necessary data
- **Local Processing**: Process sensitive data locally
- **User Control**: User control over data sharing
- **Transparency**: Clear privacy policies and practices

## Security Monitoring

### Runtime Security
- **Memory Usage**: Monitor for memory leaks in crypto operations
- **Network Anomalies**: Detect unusual network behavior
- **File System Access**: Monitor for unauthorized file access
- **Process Security**: Protect against process manipulation

### Incident Response
- **Security Incidents**: Clear incident response procedures
- **Vulnerability Reporting**: Secure vulnerability disclosure process
- **Security Updates**: Patch management and distribution
- **User Notification**: Security incident communication