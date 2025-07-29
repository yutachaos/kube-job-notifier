package monitoring

import (
	"github.com/DataDog/datadog-go/statsd"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestNewDatadog(t *testing.T) {
	os.Setenv("DD_TAGS", "tag")
	os.Setenv("DD_NAMESPACE", "namespace")

	actual := newDatadog()

	assert.Equal(t, "namespace", actual.client.Namespace)
	assert.Equal(t, []string{"tag"}, actual.client.Tags)

	os.Unsetenv("DD_TAGS")
	os.Unsetenv("DD_NAMESPACE")

	actual = newDatadog()
	assert.Empty(t, actual.client.Namespace)
	assert.Equal(t, []string{}, actual.client.Tags)
}

func TestIsSubscriptionSuppressed(t *testing.T) {
	tests := []struct {
		Name                   string
		annotations            map[string]string
		suppressAnnotationName string
		expected               bool
	}{
		{
			"No annotations",
			map[string]string{
				"kube-job-notifier/foo": "bar",
			},
			"kube-job-notifier/suppress-success-datadog-subscription",
			false,
		},
		{
			"Annotation not true",
			map[string]string{
				"kube-job-notifier/suppress-success-datadog-subscription": "false",
			},
			"kube-job-notifier/suppress-success-datadog-subscription",
			false,
		},
		{
			"Annotation true",
			map[string]string{
				"kube-job-notifier/suppress-success-datadog-subscription": "true",
			},
			"kube-job-notifier/suppress-success-datadog-subscription",
			true,
		},
		{
			"Annotation not true",
			map[string]string{
				"kube-job-notifier/suppress-failed-datadog-subscription": "false",
			},
			"kube-job-notifier/suppress-failed-datadog-subscription",
			false,
		},
		{
			"Annotation true",
			map[string]string{
				"kube-job-notifier/suppress-failed-datadog-subscription": "true",
			},
			"kube-job-notifier/suppress-failed-datadog-subscription",
			true,
		},
		{
			"Nil annotation not break",
			nil,
			"kube-job-notifier/suppress-success-datadog-subscription",
			false,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			result := isSubscriptionSuppressed(test.annotations, test.suppressAnnotationName)

			assert.Equal(t, test.expected, result)
		})
	}
}

// MockStatsdClient は statsd.Client をモックするための構造体
type MockStatsdClient struct {
	ServiceChecks []statsd.ServiceCheck
	LastError     error
	Tags          []string
	Namespace     string
}

func (m *MockStatsdClient) ServiceCheck(sc *statsd.ServiceCheck) error {
	if m.LastError != nil {
		return m.LastError
	}
	m.ServiceChecks = append(m.ServiceChecks, *sc)
	return nil
}

// mockDatadog は テスト用のDatadog構造体
type mockDatadog struct {
	client *MockStatsdClient
}

func newMockDatadog() mockDatadog {
	return mockDatadog{
		client: &MockStatsdClient{
			ServiceChecks: make([]statsd.ServiceCheck, 0),
		},
	}
}

func (d mockDatadog) SuccessEvent(jobInfo JobInfo) (err error) {
	if isSubscriptionSuppressed(jobInfo.Annotations, suppressSuccessAnnotationName) {
		return nil
	}
	sc := &statsd.ServiceCheck{
		Name:     serviceCheckName,
		Status:   statsd.Ok,
		Message:  "Job succeed",
		Hostname: hostName,
		Tags: []string{
			"job_name:" + jobInfo.getJobName(),
			"namespace:" + jobInfo.Namespace,
		},
	}
	return d.client.ServiceCheck(sc)
}

func (d mockDatadog) FailEvent(jobInfo JobInfo) (err error) {
	if isSubscriptionSuppressed(jobInfo.Annotations, suppressFailedAnnotationName) {
		return nil
	}
	sc := &statsd.ServiceCheck{
		Name:     serviceCheckName,
		Status:   statsd.Critical,
		Message:  "Job failed",
		Hostname: hostName,
		Tags: []string{
			"job_name:" + jobInfo.getJobName(),
			"namespace:" + jobInfo.Namespace,
		},
	}
	return d.client.ServiceCheck(sc)
}

func TestDatadogSuccessEvent(t *testing.T) {
	tests := []struct {
		name     string
		jobInfo  JobInfo
		expected bool // Should event be sent
	}{
		{
			name: "Success event should be sent",
			jobInfo: JobInfo{
				Name:        "test-job",
				Namespace:   "default",
				Annotations: map[string]string{},
			},
			expected: true,
		},
		{
			name: "Success event should be suppressed",
			jobInfo: JobInfo{
				Name:      "test-job",
				Namespace: "default",
				Annotations: map[string]string{
					suppressSuccessAnnotationName: "true",
				},
			},
			expected: false,
		},
		{
			name: "Success event with CronJob name",
			jobInfo: JobInfo{
				Name:        "test-job-123",
				CronJobName: "test-cronjob",
				Namespace:   "test-ns",
				Annotations: map[string]string{},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockDatadog()
			err := mock.SuccessEvent(tt.jobInfo)

			assert.NoError(t, err)

			if tt.expected {
				assert.Len(t, mock.client.ServiceChecks, 1)
				sc := mock.client.ServiceChecks[0]
				assert.Equal(t, serviceCheckName, sc.Name)
				assert.Equal(t, statsd.Ok, sc.Status)
				assert.Equal(t, "Job succeed", sc.Message)
				assert.Equal(t, hostName, sc.Hostname)

				expectedJobName := tt.jobInfo.getJobName()
				expectedTags := []string{
					"job_name:" + expectedJobName,
					"namespace:" + tt.jobInfo.Namespace,
				}
				assert.Equal(t, expectedTags, sc.Tags)
			} else {
				assert.Len(t, mock.client.ServiceChecks, 0)
			}
		})
	}
}

func TestDatadogFailEvent(t *testing.T) {
	tests := []struct {
		name     string
		jobInfo  JobInfo
		expected bool // Should event be sent
	}{
		{
			name: "Fail event should be sent",
			jobInfo: JobInfo{
				Name:        "test-job",
				Namespace:   "default",
				Annotations: map[string]string{},
			},
			expected: true,
		},
		{
			name: "Fail event should be suppressed",
			jobInfo: JobInfo{
				Name:      "test-job",
				Namespace: "default",
				Annotations: map[string]string{
					suppressFailedAnnotationName: "true",
				},
			},
			expected: false,
		},
		{
			name: "Fail event with CronJob name",
			jobInfo: JobInfo{
				Name:        "test-job-123",
				CronJobName: "test-cronjob",
				Namespace:   "test-ns",
				Annotations: map[string]string{},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockDatadog()
			err := mock.FailEvent(tt.jobInfo)

			assert.NoError(t, err)

			if tt.expected {
				assert.Len(t, mock.client.ServiceChecks, 1)
				sc := mock.client.ServiceChecks[0]
				assert.Equal(t, serviceCheckName, sc.Name)
				assert.Equal(t, statsd.Critical, sc.Status)
				assert.Equal(t, "Job failed", sc.Message)
				assert.Equal(t, hostName, sc.Hostname)

				expectedJobName := tt.jobInfo.getJobName()
				expectedTags := []string{
					"job_name:" + expectedJobName,
					"namespace:" + tt.jobInfo.Namespace,
				}
				assert.Equal(t, expectedTags, sc.Tags)
			} else {
				assert.Len(t, mock.client.ServiceChecks, 0)
			}
		})
	}
}

// テスト：Datadogのstatusがrecoverしないパターンを検証
func TestDatadogStatusRecoveryPatterns(t *testing.T) {
	t.Run("Same job fail then success recovery", func(t *testing.T) {
		mock := newMockDatadog()
		jobInfo := JobInfo{
			Name:        "test-job",
			Namespace:   "default",
			Annotations: map[string]string{},
		}

		// 最初に失敗イベントを送信
		err := mock.FailEvent(jobInfo)
		assert.NoError(t, err)
		assert.Len(t, mock.client.ServiceChecks, 1)
		assert.Equal(t, statsd.Critical, mock.client.ServiceChecks[0].Status)

		// 次に成功イベントを送信 - この場合、recoveryが動作するはず
		err = mock.SuccessEvent(jobInfo)
		assert.NoError(t, err)
		assert.Len(t, mock.client.ServiceChecks, 2)
		assert.Equal(t, statsd.Ok, mock.client.ServiceChecks[1].Status)

		// 同じタグが使用されているか確認 - これがrecoveryの鍵
		failTags := mock.client.ServiceChecks[0].Tags
		successTags := mock.client.ServiceChecks[1].Tags
		assert.Equal(t, failTags, successTags, "タグが一致しないとrecoveryが動作しない可能性がある")
	})

	t.Run("Different namespace same job name - potential recovery issue", func(t *testing.T) {
		mock := newMockDatadog()

		// 同じジョブ名だが異なるnamespace
		jobInfo1 := JobInfo{
			Name:        "test-job",
			Namespace:   "namespace1",
			Annotations: map[string]string{},
		}
		jobInfo2 := JobInfo{
			Name:        "test-job",
			Namespace:   "namespace2",
			Annotations: map[string]string{},
		}

		// namespace1で失敗
		err := mock.FailEvent(jobInfo1)
		assert.NoError(t, err)

		// namespace2で成功 - これはnamespace1のアラートをrecoverしない
		err = mock.SuccessEvent(jobInfo2)
		assert.NoError(t, err)

		assert.Len(t, mock.client.ServiceChecks, 2)

		// 異なるnamespaceタグを持つことを確認
		assert.NotEqual(t, mock.client.ServiceChecks[0].Tags, mock.client.ServiceChecks[1].Tags,
			"異なるnamespaceではタグが異なり、recoveryが動作しない")
	})

	t.Run("Different job name patterns - CronJob vs regular job", func(t *testing.T) {
		mock := newMockDatadog()

		// CronJobから生成されたJob
		cronJobInfo := JobInfo{
			Name:        "test-cronjob-123456",
			CronJobName: "test-cronjob",
			Namespace:   "default",
			Annotations: map[string]string{},
		}

		// 通常のJob
		regularJobInfo := JobInfo{
			Name:        "test-cronjob",
			Namespace:   "default",
			Annotations: map[string]string{},
		}

		// CronJobから生成されたJobで失敗
		err := mock.FailEvent(cronJobInfo)
		assert.NoError(t, err)

		// 通常のJobで成功
		err = mock.SuccessEvent(regularJobInfo)
		assert.NoError(t, err)

		assert.Len(t, mock.client.ServiceChecks, 2)

		// 同じjob_nameタグになることを確認（recoveryが動作するはず）
		assert.Equal(t, mock.client.ServiceChecks[0].Tags, mock.client.ServiceChecks[1].Tags,
			"CronJobNameが設定されている場合、同じjob_nameタグが使用されるべき")
	})

	t.Run("Empty fields handling", func(t *testing.T) {
		mock := newMockDatadog()

		// 空のフィールドを持つJobInfo
		jobInfo := JobInfo{
			Name:        "",
			CronJobName: "",
			Namespace:   "",
			Annotations: map[string]string{},
		}

		err := mock.FailEvent(jobInfo)
		assert.NoError(t, err)
		assert.Len(t, mock.client.ServiceChecks, 1)

		sc := mock.client.ServiceChecks[0]
		// 空のフィールドでもタグが正しく設定されることを確認
		expectedTags := []string{
			"job_name:",  // 空の値
			"namespace:", // 空の値
		}
		assert.Equal(t, expectedTags, sc.Tags)
	})

	t.Run("Consecutive fail events for same job", func(t *testing.T) {
		mock := newMockDatadog()
		jobInfo := JobInfo{
			Name:        "test-job",
			Namespace:   "default",
			Annotations: map[string]string{},
		}

		// 連続して失敗イベントを送信
		for i := 0; i < 3; i++ {
			err := mock.FailEvent(jobInfo)
			assert.NoError(t, err)
		}

		assert.Len(t, mock.client.ServiceChecks, 3)
		// すべてが同じタグとステータスを持つことを確認
		for i, sc := range mock.client.ServiceChecks {
			assert.Equal(t, statsd.Critical, sc.Status, "Event %d should be Critical", i)
			assert.Equal(t, []string{
				"job_name:test-job",
				"namespace:default",
			}, sc.Tags, "Event %d should have same tags", i)
		}
	})
}

// テスト：エラーハンドリングのパターン
func TestDatadogErrorHandling(t *testing.T) {
	t.Run("Client error during SuccessEvent", func(t *testing.T) {
		mock := newMockDatadog()
		mock.client.LastError = assert.AnError // エラーを設定

		jobInfo := JobInfo{
			Name:        "test-job",
			Namespace:   "default",
			Annotations: map[string]string{},
		}

		err := mock.SuccessEvent(jobInfo)
		assert.Error(t, err)
		assert.Equal(t, assert.AnError, err)

		// エラーでもServiceCheckが試行されることを確認
		assert.Len(t, mock.client.ServiceChecks, 0) // エラーのため追加されない
	})

	t.Run("Client error during FailEvent", func(t *testing.T) {
		mock := newMockDatadog()
		mock.client.LastError = assert.AnError // エラーを設定

		jobInfo := JobInfo{
			Name:        "test-job",
			Namespace:   "default",
			Annotations: map[string]string{},
		}

		err := mock.FailEvent(jobInfo)
		assert.Error(t, err)
		assert.Equal(t, assert.AnError, err)

		// エラーでもServiceCheckが試行されることを確認
		assert.Len(t, mock.client.ServiceChecks, 0) // エラーのため追加されない
	})
}

// テスト：追加の問題パターン
func TestDatadogAdditionalRecoveryIssues(t *testing.T) {
	t.Run("Case sensitivity in tags", func(t *testing.T) {
		mock := newMockDatadog()

		// 大文字小文字が異なるnamespace
		jobInfo1 := JobInfo{
			Name:        "test-job",
			Namespace:   "Default", // 大文字のD
			Annotations: map[string]string{},
		}
		jobInfo2 := JobInfo{
			Name:        "test-job",
			Namespace:   "default", // 小文字のd
			Annotations: map[string]string{},
		}

		// Defaultで失敗
		err := mock.FailEvent(jobInfo1)
		assert.NoError(t, err)

		// defaultで成功 - 大文字小文字の違いでrecoveryが動作しない可能性
		err = mock.SuccessEvent(jobInfo2)
		assert.NoError(t, err)

		assert.Len(t, mock.client.ServiceChecks, 2)

		// 異なるnamespaceタグを持つことを確認
		assert.NotEqual(t, mock.client.ServiceChecks[0].Tags, mock.client.ServiceChecks[1].Tags,
			"大文字小文字の違いでrecoveryが動作しない可能性がある")
	})

	t.Run("Special characters in job names", func(t *testing.T) {
		mock := newMockDatadog()

		jobInfo := JobInfo{
			Name:        "test-job-with-特殊文字",
			Namespace:   "default",
			Annotations: map[string]string{},
		}

		err := mock.FailEvent(jobInfo)
		assert.NoError(t, err)

		err = mock.SuccessEvent(jobInfo)
		assert.NoError(t, err)

		assert.Len(t, mock.client.ServiceChecks, 2)

		// 特殊文字が含まれても同じタグになることを確認
		assert.Equal(t, mock.client.ServiceChecks[0].Tags, mock.client.ServiceChecks[1].Tags,
			"特殊文字が含まれてもrecoveryが動作するべき")
	})

	t.Run("Very long job names", func(t *testing.T) {
		mock := newMockDatadog()

		// 非常に長いジョブ名
		longJobName := "very-long-job-name-that-might-cause-issues-in-datadog-tags-if-there-are-length-limits"
		jobInfo := JobInfo{
			Name:        longJobName,
			Namespace:   "default",
			Annotations: map[string]string{},
		}

		err := mock.FailEvent(jobInfo)
		assert.NoError(t, err)

		err = mock.SuccessEvent(jobInfo)
		assert.NoError(t, err)

		assert.Len(t, mock.client.ServiceChecks, 2)

		// 長い名前でも同じタグになることを確認
		assert.Equal(t, mock.client.ServiceChecks[0].Tags, mock.client.ServiceChecks[1].Tags,
			"長いジョブ名でもrecoveryが動作するべき")
	})

	t.Run("Job info with nil annotations", func(t *testing.T) {
		mock := newMockDatadog()

		jobInfo := JobInfo{
			Name:        "test-job",
			Namespace:   "default",
			Annotations: nil, // nilアノテーション
		}

		err := mock.FailEvent(jobInfo)
		assert.NoError(t, err)

		err = mock.SuccessEvent(jobInfo)
		assert.NoError(t, err)

		assert.Len(t, mock.client.ServiceChecks, 2)

		// nilアノテーションでも同じタグになることを確認
		assert.Equal(t, mock.client.ServiceChecks[0].Tags, mock.client.ServiceChecks[1].Tags,
			"nilアノテーションでもrecoveryが動作するべき")
	})

	t.Run("Mixed CronJob and regular job events", func(t *testing.T) {
		mock := newMockDatadog()

		// CronJobから生成されたJob（ランダムサフィックス付き）
		cronJobInfo1 := JobInfo{
			Name:        "backup-job-27894563",
			CronJobName: "backup-job",
			Namespace:   "default",
			Annotations: map[string]string{},
		}

		cronJobInfo2 := JobInfo{
			Name:        "backup-job-27894564", // 異なるサフィックス
			CronJobName: "backup-job",
			Namespace:   "default",
			Annotations: map[string]string{},
		}

		// 最初のCronJobインスタンスで失敗
		err := mock.FailEvent(cronJobInfo1)
		assert.NoError(t, err)

		// 異なるCronJobインスタンスで成功
		err = mock.SuccessEvent(cronJobInfo2)
		assert.NoError(t, err)

		assert.Len(t, mock.client.ServiceChecks, 2)

		// 同じCronJobNameなので同じjob_nameタグになることを確認
		assert.Equal(t, mock.client.ServiceChecks[0].Tags, mock.client.ServiceChecks[1].Tags,
			"同じCronJobから生成された異なるJobインスタンスでもrecoveryが動作するべき")
	})

	t.Run("Rapid fire events - race condition simulation", func(t *testing.T) {
		mock := newMockDatadog()

		jobInfo := JobInfo{
			Name:        "test-job",
			Namespace:   "default",
			Annotations: map[string]string{},
		}

		// 短時間で複数のイベントを送信（race conditionをシミュレート）
		// 失敗 -> 成功 -> 失敗 -> 成功
		events := []bool{false, true, false, true} // false=fail, true=success

		for i, isSuccess := range events {
			var err error
			if isSuccess {
				err = mock.SuccessEvent(jobInfo)
			} else {
				err = mock.FailEvent(jobInfo)
			}
			assert.NoError(t, err, "Event %d should not error", i)
		}

		assert.Len(t, mock.client.ServiceChecks, 4)

		// すべてのイベントが同じタグを持つことを確認
		expectedTags := []string{
			"job_name:test-job",
			"namespace:default",
		}
		for i, sc := range mock.client.ServiceChecks {
			assert.Equal(t, expectedTags, sc.Tags, "Event %d should have consistent tags", i)
		}

		// ステータスの順序を確認
		expectedStatuses := []statsd.ServiceCheckStatus{
			statsd.Critical, statsd.Ok, statsd.Critical, statsd.Ok,
		}
		for i, sc := range mock.client.ServiceChecks {
			assert.Equal(t, expectedStatuses[i], sc.Status, "Event %d should have correct status", i)
		}
	})
}
