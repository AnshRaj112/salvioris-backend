# API Endpoints and Request Bodies

## `POST /api/auth/signup`

**Handler:** `PrivacySignup`

**Request Body:**

```json
{
  "username": "string", // Type: string
  "password": "string", // Type: string
  "recovery_email": "string" // Type: string
}
```

## `POST /api/auth/signin`

**Handler:** `PrivacySignin`

**Request Body:**

```json
{
  "username": "string", // Type: string
  "password": "string" // Type: string
}
```

## `GET /api/auth/me`

**Handler:** `GetMe`

*(No request body usually required for this method)*

## `POST /api/auth/check-username`

**Handler:** `CheckUsernameAvailability`

**Request Body:**

```json
{
  "username": "string" // Type: string
}
```

## `POST /api/auth/forgot-username`

**Handler:** `ForgotUsername`

**Request Body:**

```json
{
  "recovery_email": "string" // Type: string
}
```

## `POST /api/auth/forgot-password`

**Handler:** `ForgotPassword`

**Request Body:**

```json
{
  "recovery_email": "string" // Type: string
}
```

## `POST /api/auth/reset-password`

**Handler:** `ResetPassword`

**Request Body:**

```json
{
  "token": "string", // Type: string
  "new_password": "string" // Type: string
}
```

## `POST /api/auth/user/signup`

**Handler:** `UserSignup`

**Request Body:**

```json
{
  "name": "string", // Type: string
  "email": "string", // Type: string
  "password": "string", // Type: string
  "street": "string", // Type: string
  "city": "string", // Type: string
  "state": "string", // Type: string
  "zip_code": "string", // Type: string
  "country": "string" // Type: string
}
```

## `POST /api/auth/user/signin`

**Handler:** `UserSignin`

**Request Body:**

```json
{
  "email": "string", // Type: string
  "password": "string" // Type: string
}
```

## `POST /api/auth/therapist/signup`

**Handler:** `TherapistSignup`

No specific JSON request body found/parsed (might be form-data or empty).

## `POST /api/auth/therapist/signin`

**Handler:** `TherapistSignin`

**Request Body:**

```json
{
  "email": "string", // Type: string
  "password": "string" // Type: string
}
```

## `GET /api/therapist/status`

**Handler:** `CheckTherapistStatus`

*(No request body usually required for this method)*

## `GET /api/therapist`

**Handler:** `GetTherapistByID`

*(No request body usually required for this method)*

## `POST /api/upload`

**Handler:** `UploadFile`

No specific JSON request body found/parsed (might be form-data or empty).

## `GET /api/admin/therapists/pending`

**Handler:** `GetPendingTherapists`

*(No request body usually required for this method)*

## `GET /api/admin/therapists/approved`

**Handler:** `GetApprovedTherapists`

*(No request body usually required for this method)*

## `PUT /api/admin/therapists/approve`

**Handler:** `ApproveTherapist`

No specific JSON request body found/parsed (might be form-data or empty).

## `DELETE /api/admin/therapists/reject`

**Handler:** `RejectTherapist`

*(No request body usually required for this method)*

## `GET /api/admin/violations`

**Handler:** `GetViolations`

*(No request body usually required for this method)*

## `GET /api/admin/blocked-ips`

**Handler:** `GetBlockedIPs`

*(No request body usually required for this method)*

## `PUT /api/admin/unblock-ip`

**Handler:** `UnblockIP`

No specific JSON request body found/parsed (might be form-data or empty).

## `GET /api/admin/users`

**Handler:** `GetUsers`

*(No request body usually required for this method)*

## `DELETE /api/admin/users`

**Handler:** `DeleteUser`

*(No request body usually required for this method)*

## `GET /api/admin/groups`

**Handler:** `AdminGetAllGroups`

*(No request body usually required for this method)*

## `GET /api/admin/groups/members`

**Handler:** `AdminGetGroupMembers`

*(No request body usually required for this method)*

## `DELETE /api/admin/groups`

**Handler:** `AdminDeleteGroup`

*(No request body usually required for this method)*

## `GET /api/admin/insights`

**Handler:** `GetInsights`

*(No request body usually required for this method)*

## `POST /api/activity`

**Handler:** `RecordActivity`

**Request Body:**

```json
{
  "path": "string" // Type: string
}
```

## `POST /api/vent`

**Handler:** `CreateVent`

**Request Body:**

```json
{
  "message": "string", // Type: string
  "user_id": "string" // Type: string
}
```

## `GET /api/vent`

**Handler:** `GetVents`

*(No request body usually required for this method)*

## `POST /api/feedback`

**Handler:** `SubmitFeedback`

**Request Body:**

```json
{
  "feedback": "string" // Type: string
}
```

## `GET /api/admin/feedbacks`

**Handler:** `GetFeedbacks`

*(No request body usually required for this method)*

## `DELETE /api/admin/feedbacks`

**Handler:** `DeleteFeedback`

*(No request body usually required for this method)*

## `POST /api/journals`

**Handler:** `CreateJournal`

**Request Body:**

```json
{
  "title": "string", // Type: string
  "content": "string", // Type: string
  "user_id": "string" // Type: string
}
```

## `GET /api/journals`

**Handler:** `GetJournals`

*(No request body usually required for this method)*

## `POST /api/contact`

**Handler:** `SubmitContact`

**Request Body:**

```json
{
  "name": "string", // Type: string
  "email": "string", // Type: string
  "message": "string" // Type: string
}
```

## `GET /api/admin/contacts`

**Handler:** `GetContacts`

*(No request body usually required for this method)*

## `DELETE /api/admin/contacts`

**Handler:** `DeleteContact`

*(No request body usually required for this method)*

## `POST /api/waitlist/user`

**Handler:** `SubmitUserWaitlist`

**Request Body:**

```json
{
  "name": "string", // Type: string
  "email": "string" // Type: string
}
```

## `POST /api/waitlist/therapist`

**Handler:** `SubmitTherapistWaitlist`

**Request Body:**

```json
{
  "name": "string", // Type: string
  "email": "string", // Type: string
  "phone": "string" // Type: string
}
```

## `GET /api/admin/waitlist/user`

**Handler:** `GetUserWaitlist`

*(No request body usually required for this method)*

## `GET /api/admin/waitlist/therapist`

**Handler:** `GetTherapistWaitlist`

*(No request body usually required for this method)*

## `DELETE /api/admin/waitlist/user`

**Handler:** `DeleteUserWaitlistEntry`

*(No request body usually required for this method)*

## `DELETE /api/admin/waitlist/therapist`

**Handler:** `DeleteTherapistWaitlistEntry`

*(No request body usually required for this method)*

## `POST /api/admin/signup`

**Handler:** `AdminSignup`

**Request Body:**

```json
{
  "username": "string", // Type: string
  "email": "string", // Type: string
  "password": "string" // Type: string
}
```

## `POST /api/admin/signin`

**Handler:** `AdminSignin`

**Request Body:**

```json
{
  "username": "string", // Type: string
  "password": "string" // Type: string
}
```

## `POST /api/groups`

**Handler:** `CreateGroup`

**Request Body:**

```json
{
  "name": "string", // Type: string
  "description": "string", // Type: string
  "tags": "string" // Type: []string
}
```

## `GET /api/groups`

**Handler:** `GetGroups`

*(No request body usually required for this method)*

## `PUT /api/groups`

**Handler:** `UpdateGroup`

**Request Body:**

```json
{
  "id": "string", // Type: string
  "name": "string", // Type: string
  "description": "string", // Type: string
  "tags": "string" // Type: []string
}
```

## `DELETE /api/groups`

**Handler:** `DeleteGroup`

*(No request body usually required for this method)*

## `POST /api/groups/join`

**Handler:** `JoinGroup`

No specific JSON request body found/parsed (might be form-data or empty).

## `DELETE /api/groups/member`

**Handler:** `RemoveMember`

*(No request body usually required for this method)*

## `GET /api/groups/members`

**Handler:** `GetGroupMembers`

*(No request body usually required for this method)*

## `GET /api/chat/history`

**Handler:** `LoadChatHistory`

*(No request body usually required for this method)*

## `GET /ws/chat`

**Handler:** `ChatWebSocket`

*(No request body usually required for this method)*

