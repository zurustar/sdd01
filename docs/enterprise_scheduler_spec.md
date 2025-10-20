# Enterprise Scheduler MVP Specification

## 1. Purpose and Scope
- **Product goal:** Provide an enterprise-grade scheduling tool that allows employees of the same company to manage and share schedules in a weekly planner style interface.
- **MVP scope:** Authentication, schedule management (create/read/update/delete), shared viewing, meeting room catalog, and repeated events as defined in this document.
- **Out of scope for MVP:** Notifications, approval workflows, external integrations, advanced security policies, and fully realized audit logging (see §8 Backlog).

## 2. Personas and Roles
| Role | Description | Capabilities |
| --- | --- | --- |
| Employee User | Any authenticated employee. | Manage their own schedules, register schedules for other employees, view other employees' schedules (subject to selection), edit schedules they created (even if for others). |
| Administrator | Employee with elevated privileges. | All employee capabilities plus user account management and meeting room catalog management. |

### Access Control Rules
- A schedule entry may be edited or deleted only by its creator or an administrator.
- Any user can create schedules on behalf of other users; ownership is tied to the creator.
- Meeting room records can be created, updated, and deleted **only** by administrators.

## 3. Schedule Data Model
Each schedule entry must capture:
- **Title** (required, text)
- **Start datetime** (required, timezone: JST)
- **End datetime** (required, must be after start)
- **Participants** (required, list of employee accounts)
- **Creator** (required, employee account reference; immutable once set)
- **Memo** (optional, rich text not required in MVP)
- **Physical meeting room** (optional, reference to meeting room catalog)
- **Web conference URL** (optional, valid URL)
- Both physical room and URL may be set simultaneously to support hybrid meetings.

### Meeting Room Catalog
Meeting rooms maintain the following attributes:
- Name (required)
- Location (required)
- Capacity (required, positive integer)
- Facilities (optional, free-form text list)

## 4. Views and Navigation
- **Supported calendar views:** day, week, month.
- Default view on login: personal week view.
- Week view is Monday-start.
- Users can toggle between "My schedule" and "Selected colleagues" mode in each view.
- When multi-user mode is active, the UI allows selecting multiple employees whose schedules will be overlaid or juxtaposed.
- Time axis defaults to 08:00–18:00 but supports vertical scrolling to show the full 24-hour range.

### 4.1 UI Components (Client Reference)
- **ビュー切り替えドロップダウン**: 週 / 日 / 月を即時切り替え。状態はクライアント側で保持し、API 呼び出しは必要に応じて `GET /schedules?view=` パラメータを変更する。
- **参加者セレクター**: ユーザー一覧をチェックボックスで提示。最大 5 名まで同時表示。選択状態は URL クエリ `participants=alice,bob` と同期し、共有リンク生成に利用する。
- **会議室ピッカー**: `GET /rooms` のレスポンスをキャッシュし、フォーム上で検索・絞り込みを行う。選択した会議室は予定カードにも `(会議室名)` と表示する。
- **競合警告バナー**: `POST /schedules` や `PUT /schedules/{id}` のレスポンスで `warnings[]` を受け取り、画面下部のバナー領域にスタック表示する。ユーザーが閉じても `警告ログ` ドロワーに履歴を保持する。
- **参加者凡例パネル**: 予定カードの色と参加者を紐づけて表示。表示順は選択順とする。

### 4.2 View Switching Validation
- 週→日→月と切り替えた場合もスクロール位置とタイムゾーン表示は維持されること。
- マルチユーザー表示時、日表示では選択したユーザーごとに縦カラムが増え、週表示では同じ時間帯に重なる予定をカードを少し重ねた形で描画する。
- 会議室選択状態はビュー切替後も予約フォームに保持される。別ビューで同会議室が重複した場合は競合警告を再表示する。
- ビュー切替・参加者更新・会議室変更は操作ごとに最大全 1 リクエスト (`GET /schedules`) で完結する設計とし、クライアントはローディングインジケーターを表示する。

## 5. Core Functional Requirements
1. **Authentication**
   - Users authenticate via email + password form.
   - No additional password policies in MVP.
2. **Schedule CRUD**
   - Users can create, read, update, and delete schedules they created.
   - Users can create schedules for other employees. Those schedules remain editable by the creator and administrators.
   - Editing another employee's schedule that the user did not create is prohibited (unless the user is an administrator).
3. **Meeting Room Management**
   - Administrators can maintain the meeting room catalog (create, edit, delete rooms).
   - Users can select rooms from the catalog when creating schedules.
4. **Recurring Schedules**
   - Support weekly recurrence.
   - Support choosing specific weekdays (multiple selection) for recurrence.
5. **Conflict Handling**
   - When a schedule conflicts by participant or meeting room, surface a warning to the creator/updater.
   - Conflicts do **not** block creation or updates in MVP.

## 6. Acceptance Criteria (BDD-Style Scenarios)

### 6.1 Viewing Personal Weekly Schedule
```gherkin
Scenario: Employee views their weekly calendar
  Given Alice is an authenticated employee
  And Alice has schedules in the current week
  When Alice opens the scheduler
  Then the week view starting Monday is displayed
  And only Alice's schedules are visible by default
  And the time axis initially spans 08:00 to 18:00 with vertical scrolling available
```

### 6.2 Viewing Multiple Employees
```gherkin
Scenario: Employee views multiple colleagues' schedules
  Given Alice, Bob, and Carol are authenticated employees
  And Alice has selected Bob and Carol in the multi-user toggle
  When Alice views the day, week, or month view
  Then Alice sees schedules for Alice, Bob, and Carol simultaneously
```

### 6.3 Creating a Schedule for Another Employee
```gherkin
Scenario: Employee creates a schedule for a colleague
  Given Alice is authenticated
  And Bob is a valid employee
  When Alice creates a schedule for Bob titled "Project Kickoff"
  Then the schedule is attributed to creator Alice and participant Bob
  And Alice can later edit or delete the schedule
  And any other non-administrator employee cannot edit or delete that schedule
```

### 6.4 Administrator Managing Meeting Rooms
```gherkin
Scenario: Administrator updates meeting room information
  Given Dana is an administrator
  And "Conference Room A" exists
  When Dana updates the capacity and facilities for "Conference Room A"
  Then the meeting room catalog reflects the new information
```

### 6.5 Recurring Weekly Schedule
```gherkin
Scenario: Employee sets a recurring weekly schedule
  Given Alice is authenticated
  When Alice creates a schedule with recurrence on Mondays and Thursdays
  Then the system generates linked occurrences for each selected weekday
  And Alice receives warnings if any generated occurrence conflicts with existing schedules but can proceed
```

## 7. UI/UX Considerations
- Planner layout should resemble a traditional weekly planner for consistency.
- Ensure sufficient contrast and usability but no specific brand guidelines are mandated for MVP.
- Interface language: Japanese only.

## 8. Backlog / Deferred Requirements
- **Audit logging:** Not part of MVP. Design should maintain creator/updater timestamps and keep architecture extensible for future logging of create/update/delete events.
- **Advanced security policies:** Password rotation, MFA, and session hardening are future considerations.
- **External integrations:** Sync with HR systems or external calendars deferred.
- **Non-functional baselines:** Performance targets, backup/DR strategy, and availability SLAs are pending and will be defined later.

## 9. Assumptions
- All users belong to the same company tenant; cross-company scheduling is out of scope.
- System operates in Japan Standard Time (UTC+9) only.
- Infrastructure for email delivery, push notifications, and similar services is not required in MVP.

## 10. Open Questions / To Clarify Later
- Finalize non-functional requirements (performance, backup, availability).
- Determine audit log retention and access policies when implemented.
- Decide on eventual password policy or SSO integration roadmap.
