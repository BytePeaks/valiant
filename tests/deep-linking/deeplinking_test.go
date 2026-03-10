package deeplinking_test

import (
	"context"
	"testing"
	"valiant/internal/config"
	"valiant/internal/domain"
	"valiant/internal/storage"
	"valiant/tests/deep-linking/shared"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLinkGeneration(t *testing.T) {
	ctx := context.Background()

	type testCase struct {
		name          string
		config        *config.Config
		events        []domain.ChangeEvent
		expectedLinks map[string]map[string]string
	}
	testCases := []testCase{
		{
			name: "No linking config",
			config: &config.Config{
				Linking: []config.LinkTemplate{},
			},
			events: []domain.ChangeEvent{
				{ID: "event-1", Metadata: map[string]string{"git_commit_sha": "abc", "repository_url": "http://repo.com"}},
			},
			expectedLinks: map[string]map[string]string{
				"event-1": {},
			},
		},
		{
			name: "Matching metadata for single link",
			config: &config.Config{
				Linking: []config.LinkTemplate{
					{Name: "View Commit", MetadataHas: []string{"git_commit_sha", "repository_url"}, URLTemplate: "{{.repository_url}}/commit/{{.git_commit_sha}}"},
				},
			},
			events: []domain.ChangeEvent{
				{ID: "event-2", Metadata: map[string]string{"git_commit_sha": "def", "repository_url": "http://repo.com/project"}},
			},
			expectedLinks: map[string]map[string]string{
				"event-2": {"View Commit": "http://repo.com/project/commit/def"},
			},
		},
		{
			name: "Partial matching metadata, no link generated",
			config: &config.Config{
				Linking: []config.LinkTemplate{
					{Name: "View Commit", MetadataHas: []string{"git_commit_sha", "repository_url"}, URLTemplate: "{{.repository_url}}/commit/{{.git_commit_sha}}"},
				},
			},
			events: []domain.ChangeEvent{
				{ID: "event-3", Metadata: map[string]string{"git_commit_sha": "ghi"}}, // Missing repository_url
			},
			expectedLinks: map[string]map[string]string{
				"event-3": {},
			},
		},
		{
			name: "Multiple link templates, one match per event",
			config: &config.Config{
				Linking: []config.LinkTemplate{
					{Name: "View Commit", MetadataHas: []string{"git_commit_sha", "repository_url"}, URLTemplate: "{{.repository_url}}/commit/{{.git_commit_sha}}"},
					{Name: "View Jenkins", MetadataHas: []string{"jenkins_build_id", "jenkins_url"}, URLTemplate: "{{.jenkins_url}}/job/{{.jenkins_build_id}}"},
				},
			},
			events: []domain.ChangeEvent{
				{ID: "event-4", Metadata: map[string]string{"git_commit_sha": "jkl", "repository_url": "http://repo.com/another"}},
			},
			expectedLinks: map[string]map[string]string{
				"event-4": {"View Commit": "http://repo.com/another/commit/jkl"},
			},
		},
		{
			name: "Multiple link templates, multiple matches per event",
			config: &config.Config{
				Linking: []config.LinkTemplate{
					{Name: "View Commit", MetadataHas: []string{"git_commit_sha", "repository_url"}, URLTemplate: "{{.repository_url}}/commit/{{.git_commit_sha}}"},
					{Name: "View Jenkins", MetadataHas: []string{"jenkins_build_id", "jenkins_url"}, URLTemplate: "{{.jenkins_url}}/job/{{.jenkins_build_id}}"},
					{Name: "View ArgoCD App", MetadataHas: []string{"argocd_url", "argocd_app_name"}, URLTemplate: "{{.argocd_url}}/applications/{{.argocd_app_name}}"},
				},
			},
			events: []domain.ChangeEvent{
				{ID: "event-5", Metadata: map[string]string{
					"git_commit_sha": "mno", "repository_url": "http://repo.com/multi",
					"jenkins_build_id": "123", "jenkins_url": "http://jenkins.com",
					"argocd_url": "http://argocd.com", "argocd_app_name": "my-app",
				}},
			},
			expectedLinks: map[string]map[string]string{
				"event-5": {
					"View Commit":     "http://repo.com/multi/commit/mno",
					"View Jenkins":    "http://jenkins.com/job/123",
					"View ArgoCD App": "http://argocd.com/applications/my-app",
				}},
		},
		{
			name: "Invalid URL template should skip link generation for that template",
			config: &config.Config{
				Linking: []config.LinkTemplate{
					{Name: "Bad Template", MetadataHas: []string{"val"}, URLTemplate: "{{.val"}, // Malformed template
					{Name: "Good Link", MetadataHas: []string{"val"}, URLTemplate: "http://example.com/{{.val}}"},
				},
			},
			events: []domain.ChangeEvent{
				{ID: "event-6", Metadata: map[string]string{"val": "test"}},
			},
			expectedLinks: map[string]map[string]string{
				"event-6": {"Good Link": "http://example.com/test"},
			},
		},
		{
			name: "Template with missing metadata key (even if not in MetadataHas) should still fail template execution",
			config: &config.Config{
				Linking: []config.LinkTemplate{
					{Name: "Missing Key In Template", MetadataHas: []string{"repo"}, URLTemplate: "http://example.com/{{.repo}}/{{.missing_key}}"},
					{Name: "Good Link", MetadataHas: []string{"repo"}, URLTemplate: "http://example.com/{{.repo}}/present"},
				},
			},
			events: []domain.ChangeEvent{
				{ID: "event-7", Metadata: map[string]string{"repo": "test"}},
			},
			expectedLinks: map[string]map[string]string{
				"event-7": {"Good Link": "http://example.com/test/present"},
			},
		},
		{
			name: "Multiple events, different link results",
			config: &config.Config{
				Linking: []config.LinkTemplate{
					{Name: "View Commit", MetadataHas: []string{"git_commit_sha", "repository_url"}, URLTemplate: "{{.repository_url}}/commit/{{.git_commit_sha}}"},
					{Name: "View Jenkins", MetadataHas: []string{"jenkins_build_id", "jenkins_url"}, URLTemplate: "{{.jenkins_url}}/job/{{.jenkins_build_id}}"},
				},
			},
			events: []domain.ChangeEvent{
				{ID: "event-8", Metadata: map[string]string{"git_commit_sha": "aaa", "repository_url": "http://git.com"}},
				{ID: "event-9", Metadata: map[string]string{"jenkins_build_id": "bbb", "jenkins_url": "http://jenkins.org"}},
				{ID: "event-10", Metadata: map[string]string{"foo": "bar"}}, // No matching links
			},
			expectedLinks: map[string]map[string]string{
				"event-8":  {"View Commit": "http://git.com/commit/aaa"},
				"event-9":  {"View Jenkins": "http://jenkins.org/job/bbb"},
				"event-10": {},
			},
		},
		{
			name: "Empty MetadataHas, static and dynamic links",
			config: &config.Config{
				Linking: []config.LinkTemplate{
					{Name: "Static Link", MetadataHas: []string{}, URLTemplate: "http://static.example.com"},
					{Name: "Dynamic Link", MetadataHas: []string{}, URLTemplate: "http://dynamic.example.com/{{.some_key}}"},
				},
			},
			events: []domain.ChangeEvent{
				{ID: "event-11", Metadata: map[string]string{"some_key": "value"}},
				{ID: "event-12", Metadata: map[string]string{}}, // For dynamic link, it will produce <no value>
			},
			expectedLinks: map[string]map[string]string{
				"event-11": {
					"Static Link":  "http://static.example.com",
					"Dynamic Link": "http://dynamic.example.com/value",
				},
				"event-12": {
					"Static Link": "http://static.example.com",
					// Dynamic Link will be skipped due to <no value> check.
				},
			},
		},
		{
			name: "Metadata value with special characters (no URL encoding)",
			config: &config.Config{
				Linking: []config.LinkTemplate{
					{Name: "Special Char Link", MetadataHas: []string{"path"}, URLTemplate: "http://host/resource/{{.path}}"},
				},
			},
			events: []domain.ChangeEvent{
				{ID: "event-13", Metadata: map[string]string{"path": "foo/bar?param=value&name=test"}},
			},
			expectedLinks: map[string]map[string]string{
				"event-13": {"Special Char Link": "http://host/resource/foo/bar?param=value&name=test"},
			},
		},
		{
			name: "Non-string metadata value (integer)",
			config: &config.Config{
				Linking: []config.LinkTemplate{
					{Name: "Int Link", MetadataHas: []string{"version"}, URLTemplate: "http://app.example.com/v{{.version}}"},
				},
			},
			events: []domain.ChangeEvent{
				{ID: "event-14", Metadata: map[string]string{"version": "123"}}, // int value
			},
			expectedLinks: map[string]map[string]string{
				"event-14": {"Int Link": "http://app.example.com/v123"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db, schemaName, err := shared.SetupTestDB() // Corrected call
			require.NoError(t, err)
			defer shared.CleanupTestDB(db, schemaName)

			store := storage.NewPostgresStorage(db, tc.config) // Corrected call

			for _, event := range tc.events {
				if err := store.SaveChangeEvent(ctx, event); err != nil {
					t.Fatalf("Failed to save event %s: %v", event.ID, err)
				}
			}

			retrievedEvents, err := store.GetEventsPendingAnalysis(ctx)
			require.NoError(t, err)

			// Create a map for easier assertion
			eventLinksMap := make(map[string]map[string]string)
			for _, event := range retrievedEvents {
				links := make(map[string]string)
				for _, link := range event.ContextualLinks {
					links[link.Name] = link.URL
				}
				eventLinksMap[event.ID] = links
			}

			// Assertions
			// The number of retrieved events might differ if some events were not saved or filtered,
			// but here we expect all events to be retrieved and processed.
			assert.Equal(t, len(tc.expectedLinks), len(eventLinksMap), "Number of events with links mismatch")

			for eventID, expectedLinks := range tc.expectedLinks {
				actualLinks, ok := eventLinksMap[eventID]
				require.True(t, ok, "Event ID %s not found in retrieved events", eventID)

				assert.Equal(t, len(expectedLinks), len(actualLinks), "Event %s: Number of links mismatch. Expected: %v, Actual: %v", eventID, expectedLinks, actualLinks)

				for name, expectedURL := range expectedLinks {
					actualURL, found := actualLinks[name]
					assert.True(t, found, "Event %s: Expected link '%s' not found", eventID, name)
					assert.Equal(t, expectedURL, actualURL, "Event %s, link '%s': URL mismatch", eventID, name)
				}
			}
		})
	}
}
