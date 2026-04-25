package auth

import (
	"sync"
	"testing"
	"time"
)

type blockingAuthPersistence struct {
	*stubAuthPersistence

	mu            sync.Mutex
	blockFindByID bool
	startedCh     chan struct{}
	releaseCh     chan struct{}
}

func newBlockingAuthPersistence() *blockingAuthPersistence {
	return &blockingAuthPersistence{
		stubAuthPersistence: newStubAuthPersistence(),
		startedCh:           make(chan struct{}, 8),
		releaseCh:           make(chan struct{}),
	}
}

func (s *blockingAuthPersistence) setBlockFindByID(block bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.blockFindByID = block
}

func (s *blockingAuthPersistence) releaseBlockedFindByID() {
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case <-s.releaseCh:
		return
	default:
		close(s.releaseCh)
	}
}

func (s *blockingAuthPersistence) FindAuthUserByID(userID string) (User, []byte, bool, error) {
	s.mu.Lock()
	block := s.blockFindByID
	startedCh := s.startedCh
	releaseCh := s.releaseCh
	s.mu.Unlock()

	if block {
		select {
		case startedCh <- struct{}{}:
		default:
		}
		<-releaseCh
	}

	return s.stubAuthPersistence.FindAuthUserByID(userID)
}

func TestVerifyAccessTokenConcurrentReadsDoNotSerializeOnServiceMutex(t *testing.T) {
	store := newBlockingAuthPersistence()
	svc := NewService("", "", 0, 0, true)
	if err := svc.SetPersistence(store); err != nil {
		t.Fatalf("set persistence failed: %v", err)
	}

	login, err := svc.Login(LoginRequest{
		Email:    "tenant.admin@sudirman.co",
		Password: "admin123",
	})
	if err != nil {
		t.Fatalf("seed login failed: %v", err)
	}

	store.setBlockFindByID(true)
	defer store.releaseBlockedFindByID()

	errCh := make(chan error, 2)
	for range 2 {
		go func() {
			_, verifyErr := svc.VerifyAccessToken(login.AccessToken)
			errCh <- verifyErr
		}()
	}

	for i := 0; i < 2; i++ {
		select {
		case <-store.startedCh:
		case <-time.After(time.Second):
			t.Fatalf("expected concurrent verify requests to reach persistence read #%d", i+1)
		}
	}

	store.releaseBlockedFindByID()

	for i := 0; i < 2; i++ {
		select {
		case verifyErr := <-errCh:
			if verifyErr != nil {
				t.Fatalf("verify access token failed: %v", verifyErr)
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for verify result #%d", i+1)
		}
	}
}
