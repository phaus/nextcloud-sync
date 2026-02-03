# API Specification

## Overview

This document defines the API interfaces and protocols used by the Nextcloud Sync CLI tool.

## WebDAV Integration

### Base WebDAV Endpoint
```
https://<nextcloud-domain>/remote.php/dav/files/<username>/
```

### Key WebDAV Methods

#### PROPFIND
Used for retrieving file metadata and directory listings.

**Request:**
```
PROPFIND /remote.php/dav/files/username/path/to/file HTTP/1.1
Depth: 1
Authorization: Basic <base64(username:app_password)>
```

**Response Properties:**
- `d:getlastmodified` - File modification timestamp
- `d:getetag` - Entity tag for change detection
- `d:getcontentlength` - File size
- `d:resourcetype` - Directory vs file identification

#### GET
Download file content from Nextcloud.

**Request:**
```
GET /remote.php/dav/files/username/path/to/file HTTP/1.1
Authorization: Basic <base64(username:app_password)>
```

#### PUT
Upload file content to Nextcloud.

**Request:**
```
PUT /remote.php/dav/files/username/path/to/file HTTP/1.1
Authorization: Basic <base64(username:app_password)>
Content-Length: <file_size>
```

#### MKCOL
Create directories in Nextcloud.

**Request:**
```
MKCOL /remote.php/dav/files/username/path/to/directory HTTP/1.1
Authorization: Basic <base64(username:app_password)>
```

#### DELETE
Remove files and directories from Nextcloud.

**Request:**
```
DELETE /remote.php/dav/files/username/path/to/file HTTP/1.1
Authorization: Basic <base64(username:app_password)>
```

## Authentication

### App Password Authentication
- Uses Nextcloud's app password feature
- Format: HTTP Basic Authentication with `username:app_password`
- Password must be generated in Nextcloud security settings

### App Password Generation
1. Navigate to Nextcloud Settings → Security
2. Click "Create new app password"
3. Label with device/application name
4. Use generated password for API authentication

## Error Handling

### HTTP Status Codes
- `200 OK` - Successful operation
- `201 Created` - Resource created successfully
- `204 No Content` - Operation successful, no content returned
- `207 Multi-Status` - PROPFIND response with multiple status codes
- `401 Unauthorized` - Invalid credentials
- `403 Forbidden` - Permission denied
- `404 Not Found` - Resource does not exist
- `409 Conflict` - Resource conflict (e.g., directory exists when creating file)
- `423 Locked` - Resource is locked
- `507 Insufficient Storage` - Quota exceeded

### Nextcloud-Specific Error Responses
XML error responses in the following format:
```xml
<?xml version="1.0" encoding="utf-8"?>
<d:error xmlns:d="DAV:" xmlns:s="http://sabredav.org/ns">
  <s:exception>Sabre\DAV\Exception\NotFound</s:exception>
  <s:message>File not found</s:message>
</d:error>
```

## Configuration API

### Config File Structure
Location: `~/.nextcloud-sync/config.json`

```json
{
  "version": "1.0",
  "servers": {
    "default": {
      "url": "https://cloud.example.com",
      "username": "user@example.com",
      "app_password": "encrypted_password",
      "root_path": "/files/12345"
    }
  },
  "sync_profiles": {
    "documents": {
      "source": "~/Documents",
      "target": "https://cloud.example.com/apps/files/files/12345?dir=/Documents",
      "exclude_patterns": ["*.tmp", ".DS_Store"],
      "bidirectional": true
    }
  },
  "global_settings": {
    "max_retries": 3,
    "timeout_seconds": 30,
    "chunk_size_mb": 50,
    "progress_update_interval_ms": 1000
  }
}
```

## CLI Command API

### Primary Command
```bash
agent <source> <target> [options]
```

### Options
- `--dry-run` - Show what would be synced without making changes
- `--force` - Force overwrite conflicting files
- `--exclude=PATTERN` - Additional exclude patterns
- `--profile=NAME` - Use predefined sync profile
- `--verbose` - Detailed logging output
- `--config=PATH` - Custom config file location

### Exit Codes
- `0` - Success
- `1` - General error
- `2` - Authentication failed
- `3` - Network connectivity issue
- `4` - Configuration error
- `5` - File system error
- `6` - Permission denied

## Change Detection API

### Nextcloud Properties Used
- `getlastmodified` - Last modification timestamp
- `getetag` - Entity tag for content verification
- `getcontentlength` - File size comparison

### Change Detection Algorithm
1. Compare local vs remote modification timestamps
2. Verify using ETags when timestamps are equal but sizes differ
3. Handle timezone differences using Nextcloud's ISO 8601 timestamps
4. Implement conflict resolution based on "source wins" policy

## File Exclusion API

### .nextcloudignore Format
Gitignore-style pattern matching:

```
# Comment lines start with #
*.tmp
.DS_Store
.git/
node_modules/
*.log
temp/
```

### Pattern Matching Rules
- `*` matches any characters except `/`
- `**` matches any characters including `/`
- `?` matches any single character
- `[abc]` matches any character in brackets
- `[!abc]` negated character class
- Leading `/` anchors to root
- Trailing `/` matches directories only