# Deferred Services Migration

Three `apps/web/core/services/` files require significant work before migration because they
are either large catch-all services or have many missing methods in `@plane/services`.
This document captures the full research so the work can be picked up without re-analysis.

---

## 1. `workspace.service.ts`

### Why deferred

The web `WorkspaceService` is a **catch-all** — its methods are split across _five_ separate
package sub-services. Each consumer in the web app will need to import from the right
package class rather than a single `WorkspaceService`.

### Package sub-services that already cover web methods

| Web method                                                                                      | Package service              | Package method                                                          |
| ----------------------------------------------------------------------------------------------- | ---------------------------- | ----------------------------------------------------------------------- |
| `workspaceMemberMe`                                                                             | `WorkspaceMemberService`     | `myInfo`                                                                |
| `fetchWorkspaceMembers`                                                                         | `WorkspaceMemberService`     | `list`                                                                  |
| `updateWorkspaceMember`                                                                         | `WorkspaceMemberService`     | `update`                                                                |
| `deleteWorkspaceMember`                                                                         | `WorkspaceMemberService`     | `destroy`                                                               |
| `getWorkspaceUserProjectsRole`                                                                  | `WorkspaceMemberService`     | `getWorkspaceUserProjectsRole`                                          |
| `userWorkspaceInvitations`                                                                      | `WorkspaceInvitationService` | `userInvitations`                                                       |
| `workspaceInvitations`                                                                          | `WorkspaceInvitationService` | `workspaceInvitations`                                                  |
| `inviteWorkspace`                                                                               | `WorkspaceInvitationService` | `invite`                                                                |
| `updateWorkspaceInvitation`                                                                     | `WorkspaceInvitationService` | `update`                                                                |
| `deleteWorkspaceInvitations`                                                                    | `WorkspaceInvitationService` | `destroy`                                                               |
| `joinWorkspace`                                                                                 | `WorkspaceInvitationService` | `join`                                                                  |
| `joinWorkspaces`                                                                                | `WorkspaceInvitationService` | `joinMany`                                                              |
| `createView` / `updateView` / `deleteView` / `getAllViews` / `getViewDetails` / `getViewIssues` | `WorkspaceViewService`       | `create` / `update` / `destroy` / `list` / `retrieve` / `getViewIssues` |

### Already in package `WorkspaceService` with renames

| Web method                               | Package method                  |
| ---------------------------------------- | ------------------------------- |
| `userWorkspaces()`                       | `list()`                        |
| `getWorkspace(workspaceSlug)`            | `retrieve(workspaceSlug)`       |
| `createWorkspace(data)`                  | `create(data)`                  |
| `updateWorkspace(workspaceSlug, data)`   | `update(workspaceSlug, data)`   |
| `deleteWorkspace(workspaceSlug)`         | `destroy(workspaceSlug)`        |
| `getLastActiveWorkspaceAndProjects()`    | `lastVisited()`                 |
| `workspaceSlugCheck(slug)`               | `slugCheck(slug)`               |
| `searchWorkspace(workspaceSlug, params)` | `search(workspaceSlug, params)` |

### Methods missing from ALL package services — add to `WorkspaceService`

| Web method                                            | Proposed name                  | HTTP   | Endpoint                                                        |
| ----------------------------------------------------- | ------------------------------ | ------ | --------------------------------------------------------------- |
| `getWorkspaceInvitation(workspaceSlug, invitationId)` | `getInvitation`                | GET    | `/api/workspaces/${workspaceSlug}/invitations/${invitationId}/` |
| `updateWorkspaceView(workspaceSlug, {view_props})`    | `updateViewProps`              | POST   | `/api/workspaces/${workspaceSlug}/workspace-view/`              |
| `getProductUpdates()`                                 | `getProductUpdates`            | GET    | `/api/release-notes/`                                           |
| `fetchWorkspaceLinks(workspaceSlug)`                  | `listLinks`                    | GET    | `/api/workspaces/${workspaceSlug}/workspace-links/`             |
| `createWorkspaceLink(workspaceSlug, data)`            | `createLink`                   | POST   | `/api/workspaces/${workspaceSlug}/workspace-links/`             |
| `updateWorkspaceLink(workspaceSlug, linkId, data)`    | `updateLink`                   | PATCH  | `/api/workspaces/${workspaceSlug}/workspace-links/${linkId}/`   |
| `deleteWorkspaceLink(workspaceSlug, linkId)`          | `deleteLink`                   | DELETE | `/api/workspaces/${workspaceSlug}/workspace-links/${linkId}/`   |
| `searchEntity(workspaceSlug, params)`                 | `searchEntity`                 | GET    | `/api/workspaces/${workspaceSlug}/search/`                      |
| `fetchWorkspaceRecents(workspaceSlug, entity_name?)`  | `listRecents`                  | GET    | `/api/workspaces/${workspaceSlug}/recent-visits/`               |
| `fetchWorkspaceWidgets(workspaceSlug)`                | `listWidgets`                  | GET    | `/api/workspaces/${workspaceSlug}/workspace-widgets/`           |
| `updateWorkspaceWidget(dashboardId, widgetId, data)`  | `updateWidget`                 | PATCH  | `/api/dashboard/${dashboardId}/widgets/${widgetId}/`            |
| `fetchSidebarNavigationPreferences(workspaceSlug)`    | `getSidebarPreferences`        | GET    | `/api/workspaces/${workspaceSlug}/sidebar-preferences/`         |
| `updateSidebarPreference(workspaceSlug, data)`        | `updateSidebarPreference`      | PATCH  | `/api/workspaces/${workspaceSlug}/sidebar-preferences/`         |
| `updateBulkSidebarPreferences(workspaceSlug, data)`   | `updateBulkSidebarPreferences` | PUT    | `/api/workspaces/${workspaceSlug}/sidebar-preferences/`         |
| `fetchWorkspaceFilters(workspaceSlug)`                | `getUserProperties`            | GET    | `/api/workspaces/${workspaceSlug}/user-properties/`             |
| `patchWorkspaceFilters(workspaceSlug, userId, data)`  | `updateUserProperties`         | PATCH  | `/api/workspaces/${workspaceSlug}/user-properties/${userId}/`   |

### Migration steps (when ready)

1. Add the 16 missing methods to `packages/services/src/workspace/workspace.service.ts`
2. Find all consumers of `@/services/workspace.service` in `apps/web`
3. For each consumer, determine which package service class it needs:
   - Member methods → `WorkspaceMemberService`
   - Invitation methods → `WorkspaceInvitationService`
   - View methods → `WorkspaceViewService`
   - Core workspace methods → `WorkspaceService`
4. Update imports and rename methods
5. Delete `apps/web/core/services/workspace.service.ts`

---

## 2. `user.service.ts`

### Why deferred

The web `UserService` has 24 methods. The package only has 5. 20 methods need to be
added before migration is possible.

### Already in package `UserService` (rename only)

| Web method                       | Package method        | HTTP  | Endpoint                 |
| -------------------------------- | --------------------- | ----- | ------------------------ |
| `currentUser()`                  | `me()`                | GET   | `/api/users/me/`         |
| `getCurrentUserProfile()`        | `profile()`           | GET   | `/api/users/me/profile/` |
| `updateCurrentUserProfile(data)` | `updateProfile(data)` | PATCH | `/api/users/me/profile/` |
| `updateUser(data)`               | `update(data)`        | PATCH | `/api/users/me/`         |

Note: `currentUserConfig()` is a non-async utility that returns a URL config object — keep in web or add as a getter.

### Methods missing — add to `packages/services/src/user/user.service.ts`

| Web method                                                    | Proposed name                     | HTTP   | Endpoint                                                                |
| ------------------------------------------------------------- | --------------------------------- | ------ | ----------------------------------------------------------------------- |
| `userIssues(workspaceSlug, params)`                           | `listIssues`                      | GET    | `/api/workspaces/${workspaceSlug}/my-issues/`                           |
| `getCurrentUserAccounts()`                                    | `getAccounts`                     | GET    | `/api/users/me/accounts/`                                               |
| `currentUserInstanceAdminStatus()`                            | `getInstanceAdminStatus`          | GET    | `/api/users/me/instance-admin/`                                         |
| `currentUserSettings(bustCache?)`                             | `getSettings`                     | GET    | `/api/users/me/settings/`                                               |
| `currentUserEmailNotificationSettings()`                      | `getEmailNotificationSettings`    | GET    | `/api/users/me/notification-preferences/`                               |
| `updateUserOnBoard()`                                         | `updateOnboard`                   | PATCH  | `/api/users/me/onboard/` with `{is_onboarded: true}`                    |
| `updateUserTourCompleted()`                                   | `updateTourCompleted`             | PATCH  | `/api/users/me/tour-completed/` with `{is_tour_completed: true}`        |
| `updateCurrentUserEmailNotificationSettings(data)`            | `updateEmailNotificationSettings` | PATCH  | `/api/users/me/notification-preferences/`                               |
| `changePassword(token, data)`                                 | `changePassword`                  | POST   | `/auth/change-password/` (with X-CSRFTOKEN header)                      |
| `getUserProfileData(workspaceSlug, userId)`                   | `getProfileStats`                 | GET    | `/api/workspaces/${workspaceSlug}/user-stats/${userId}/`                |
| `getUserProfileProjectsSegregation(workspaceSlug, userId)`    | `getProfileProjects`              | GET    | `/api/workspaces/${workspaceSlug}/user-profile/${userId}/`              |
| `getUserProfileActivity(workspaceSlug, userId, params)`       | `getProfileActivity`              | GET    | `/api/workspaces/${workspaceSlug}/user-activity/${userId}/`             |
| `downloadProfileActivity(workspaceSlug, userId, data)`        | `downloadProfileActivity`         | POST   | `/api/workspaces/${workspaceSlug}/user-activity/${userId}/export/`      |
| `getUserProfileIssues(workspaceSlug, userId, params, config)` | `getProfileIssues`                | GET    | `/api/workspaces/${workspaceSlug}/user-issues/${userId}/`               |
| `deactivateAccount()`                                         | `deactivate`                      | DELETE | `/api/users/me/`                                                        |
| `leaveWorkspace(workspaceSlug)`                               | `leaveWorkspace`                  | POST   | `/api/workspaces/${workspaceSlug}/members/leave/`                       |
| `joinProject(workspaceSlug, project_ids)`                     | `joinProjects`                    | POST   | `/api/users/me/workspaces/${workspaceSlug}/projects/invitations/`       |
| `leaveProject(workspaceSlug, projectId)`                      | `leaveProject`                    | POST   | `/api/workspaces/${workspaceSlug}/projects/${projectId}/members/leave/` |
| `checkEmail(token, email)`                                    | `checkEmail`                      | POST   | `/auth/email-check/` (with X-CSRFTOKEN header)                          |
| `generateEmailCode(data)`                                     | `generateEmailCode`               | POST   | `/api/users/me/email/generate-code/`                                    |
| `verifyEmailCode(data)`                                       | `verifyEmailCode`                 | PATCH  | `/api/users/me/email/`                                                  |

### Migration steps (when ready)

1. Add all 21 missing methods to `packages/services/src/user/user.service.ts`
2. Add missing type imports (`IInstanceAdminStatus`, `IUserSettings`, etc.)
3. Find all consumers of `@/services/user.service` in `apps/web`
4. Update imports to `@plane/services`, rename the 4 mapped methods
5. Handle `currentUserConfig()` — either add as a getter or keep in web
6. Handle default export `userService` — package may need to export a singleton or consumers instantiate
7. Delete `apps/web/core/services/user.service.ts`

---

## 3. `file.service.ts`

### Why deferred

Several methods involve a **3-step S3 signed URL upload flow** (request signed URL → upload to S3 → confirm upload status). These internally use `FileUploadService` which is already in `@plane/services`.

### Already in package `FileService`

| Web method                                | Package method          | Status                         |
| ----------------------------------------- | ----------------------- | ------------------------------ |
| `deleteNewAsset(assetPath)`               | `deleteNewAsset`        | ✅ same                        |
| `restoreOldEditorAsset(workspaceId, src)` | `restoreOldEditorAsset` | ✅ same                        |
| `duplicateAsset(workspaceSlug, data)`     | `duplicateAssets`       | ⚠️ renamed (singular → plural) |

### Methods missing — add to `packages/services/src/file/file.service.ts`

| Web method                                                            | Proposed name                     | HTTP      | Endpoint                                                            |
| --------------------------------------------------------------------- | --------------------------------- | --------- | ------------------------------------------------------------------- |
| `uploadWorkspaceAsset(workspaceSlug, data, file, progress?)`          | `uploadWorkspaceAsset`            | POST + S3 | `/api/assets/v2/workspaces/${workspaceSlug}/`                       |
| `deleteWorkspaceAsset(workspaceSlug, assetId)`                        | `deleteWorkspaceAsset`            | DELETE    | `/api/assets/v2/workspaces/${workspaceSlug}/${assetId}/`            |
| `updateBulkWorkspaceAssetsUploadStatus(workspaceSlug, data)`          | `updateBulkWorkspaceUploadStatus` | PATCH     | `/api/assets/v2/workspaces/${workspaceSlug}/`                       |
| `updateBulkProjectAssetsUploadStatus(workspaceSlug, projectId, data)` | `updateBulkProjectUploadStatus`   | PATCH     | `/api/assets/v2/workspaces/${workspaceSlug}/projects/${projectId}/` |
| `uploadProjectAsset(workspaceSlug, projectId, data, file, progress?)` | `uploadProjectAsset`              | POST + S3 | `/api/assets/v2/workspaces/${workspaceSlug}/projects/${projectId}/` |
| `uploadUserAsset(data, file)`                                         | `uploadUserAsset`                 | POST + S3 | `/api/assets/v2/user-assets/`                                       |
| `deleteUserAsset(assetId)`                                            | `deleteUserAsset`                 | DELETE    | `/api/assets/v2/user-assets/${assetId}/`                            |
| `deleteOldWorkspaceAsset(workspaceId, src)`                           | `deleteOldWorkspaceAsset`         | DELETE    | `/api/workspaces/file-assets/${workspaceId}/${assetKey}/`           |
| `deleteOldUserAsset(src)`                                             | `deleteOldUserAsset`              | DELETE    | `/api/users/file-assets/${assetKey}/`                               |
| `restoreNewAsset(workspaceSlug, src)`                                 | `restoreNewAsset`                 | POST      | `/api/assets/v2/workspaces/${workspaceSlug}/restore/${assetId}/`    |
| `checkIfAssetExists(workspaceSlug, src)`                              | `checkIfAssetExists`              | GET       | `/api/assets/v2/workspaces/${workspaceSlug}/${assetId}/exists/`     |
| `getUnsplashImages(query?)`                                           | `getUnsplashImages`               | GET       | `/api/workspaces/unsplash-images/`                                  |

### S3 upload flow note

The upload methods (`uploadWorkspaceAsset`, `uploadProjectAsset`, `uploadUserAsset`) use this pattern:

1. POST to the asset endpoint to get a signed URL response
2. Use `FileUploadService.uploadFile(signedURL, formData, progressHandler)` to upload to S3
3. PATCH a status endpoint to confirm the upload completed

`FileUploadService` is already in `packages/services/src/file/file-upload.service.ts` and can be used directly inside the package `FileService`.

### Migration steps (when ready)

1. Add all 12 missing methods to `packages/services/src/file/file.service.ts`
2. Import `FileUploadService` from `./file-upload.service` (relative, not from `@plane/services`)
3. Import `getFileMetaDataForUpload`, `generateFileUploadPayload` from `./helper` (relative)
4. Find all consumers of `@/services/file.service` in `apps/web`
5. Update imports to `@plane/services`, rename `duplicateAsset` → `duplicateAssets`
6. Delete `apps/web/core/services/file.service.ts`
