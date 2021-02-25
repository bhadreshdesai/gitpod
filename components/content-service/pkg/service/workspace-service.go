// Copyright (c) 2021 Gitpod GmbH. All rights reserved.
// Licensed under the GNU Affero General Public License (AGPL).
// See License-AGPL.txt in the project root for license information.

package service

import (
	"context"
	"strings"

	"github.com/gitpod-io/gitpod/common-go/log"
	"github.com/gitpod-io/gitpod/common-go/tracing"
	"github.com/gitpod-io/gitpod/content-service/api"
	"github.com/gitpod-io/gitpod/content-service/pkg/storage"
	"github.com/opentracing/opentracing-go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// WorkspaceService implements WorkspaceServiceServer
type WorkspaceService struct {
	cfg storage.Config
	s   storage.PresignedAccess
}

// NewWorkspaceService create a new content service
func NewWorkspaceService(cfg storage.Config) (res *WorkspaceService, err error) {
	s, err := storage.NewPresignedAccess(&cfg)
	if err != nil {
		return nil, err
	}
	return &WorkspaceService{cfg, s}, nil
}

// DownloadUrlWorkspace provides a URL from where the content of a workspace can be downloaded from
func (cs *WorkspaceService) DownloadUrlWorkspace(ctx context.Context, req *api.DownloadUrlWorkspaceRequest) (resp *api.DownloadUrlWorkspaceResponse, err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "DownloadUrlWorkspace")
	span.SetTag("user", req.OwnerId)
	span.SetTag("workspaceId", req.WorkspaceId)
	defer tracing.FinishSpan(span, &err)

	blobName := cs.s.BackupObject(req.WorkspaceId, storage.DefaultBackup)

	info, err := cs.s.SignDownload(ctx, cs.s.Bucket(req.OwnerId), blobName, &storage.SignedURLOptions{
		// ContentType: "application/tar+gzip",
		ContentType: "*/*",
	})
	if err != nil {
		log.Error("error getting SignDownload URL: ", err)
		if err == storage.ErrNotFound {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Unknown, err.Error())
	}

	return &api.DownloadUrlWorkspaceResponse{
		Url: info.URL,
	}, nil
}

// DeleteWorkspace deletes the content of a single workspace
func (cs *WorkspaceService) DeleteWorkspace(ctx context.Context, req *api.DeleteWorkspaceRequest) (resp *api.DeleteWorkspaceResponse, err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "DeleteWorkspace")
	span.SetTag("user", req.OwnerId)
	span.SetTag("workspaceId", req.WorkspaceId)
	span.SetTag("includeSnapshots", req.IncludeSnapshots)
	defer tracing.FinishSpan(span, &err)

	if req.IncludeSnapshots {
		prefix := cs.s.BackupObject(req.WorkspaceId, "")
		if !strings.HasSuffix(prefix, "/") {
			prefix = prefix + "/"
		}
		err = cs.s.DeleteObject(ctx, cs.s.Bucket(req.OwnerId), &storage.DeleteObjectQuery{Prefix: prefix})
		if err != nil {
			log.WithError(err).Error("error deleting workspace backup")
			if err == storage.ErrNotFound {
				return nil, status.Error(codes.NotFound, err.Error())
			}
			return nil, status.Error(codes.Unknown, err.Error())
		}
	} else {
		blobName := cs.s.BackupObject(req.WorkspaceId, storage.DefaultBackup)
		err = cs.s.DeleteObject(ctx, cs.s.Bucket(req.OwnerId), &storage.DeleteObjectQuery{Name: blobName})
		if err != nil {
			log.WithError(err).Error("error deleting workspace backup: ", blobName)
			if err == storage.ErrNotFound {
				return nil, status.Error(codes.NotFound, err.Error())
			}
			return nil, status.Error(codes.Unknown, err.Error())
		}

		trailPrefix := cs.s.BackupObject(req.WorkspaceId, "trail-")
		err = cs.s.DeleteObject(ctx, cs.s.Bucket(req.OwnerId), &storage.DeleteObjectQuery{Prefix: trailPrefix})
		if err != nil {
			log.WithError(err).Error("error deleting workspace backup: ", trailPrefix)
			if err == storage.ErrNotFound {
				return nil, status.Error(codes.NotFound, err.Error())
			}
			return nil, status.Error(codes.Unknown, err.Error())
		}
	}

	return &api.DeleteWorkspaceResponse{}, nil
}
