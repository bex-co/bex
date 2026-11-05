/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// defaultCredentialRequeue paces the wait for a rotation's source Secrets.
const defaultCredentialRequeue = 10 * time.Second

// reconcileDefaultConnectionSecret projects a non-owner default credential
// (spec.defaultUser) into DefaultConnectionSecretName: CNPG's "-app" Secret
// with the default user's username and password substituted, so every
// consumer of status.secretName — connection info, Blueprint fromDatabase
// references, the pooler Secret and exports — keeps reading one key set.
//
// It returns the Secret current for the default credential, or "" while a
// source Secret does not exist yet (CNPG has not generated "-app", or
// bex-api's user Secret is still being written); the caller then keeps the
// previous status.secretName rather than falling back to the owner, whose
// login may already be retired.
func (r *DatabaseReconciler) reconcileDefaultConnectionSecret(ctx context.Context, db *appv1alpha1.Database) (string, error) {
	target := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Name: appv1alpha1.DefaultConnectionSecretName(db.Name), Namespace: db.Namespace,
	}}
	if db.Spec.DefaultUser == "" {
		// Only a database that was rotated has the Secret to remove; the
		// rest read nothing here.
		if db.Status.SecretName == target.Name {
			if err := r.secretClient().Delete(ctx, target); client.IgnoreNotFound(err) != nil {
				return "", err
			}
		}
		return appv1alpha1.ApplicationSecretName(db.Name), nil
	}
	var userSecretName string
	for _, u := range db.Spec.Users {
		if u.Name == db.Spec.DefaultUser {
			userSecretName = u.SecretName
		}
	}
	if userSecretName == "" {
		return "", fmt.Errorf("default user %q has no credential Secret", db.Spec.DefaultUser)
	}
	source, ok, err := r.optionalSecret(ctx, db.Namespace, appv1alpha1.ApplicationSecretName(db.Name))
	if !ok || err != nil {
		return "", err
	}
	userSecret, ok, err := r.optionalSecret(ctx, db.Namespace, userSecretName)
	if !ok || err != nil {
		return "", err
	}
	password := userSecret.Data["password"]
	if len(password) == 0 {
		return "", fmt.Errorf("credential Secret %q lacks a password", userSecretName)
	}
	data, err := defaultConnectionData(source.Data, db.Spec.DefaultUser, string(password))
	if err != nil {
		return "", err
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.secretClient(), target, func() error {
		target.Type = corev1.SecretTypeOpaque
		target.Data = data
		return controllerutil.SetControllerReference(db, target, r.Scheme)
	}); err != nil {
		return "", err
	}
	return target.Name, nil
}

func (r *DatabaseReconciler) optionalSecret(ctx context.Context, namespace, name string) (*corev1.Secret, bool, error) {
	sec := &corev1.Secret{}
	if err := r.secretClient().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, sec); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return sec, true, nil
}

// defaultConnectionData rewrites CNPG's application-Secret keys for another
// login: the plain fields, the URL-shaped URIs, the JDBC URIs (credentials in
// the query string) and the pgpass line. Keys bex does not know are copied
// unchanged unless they carry the owner's password, which never leaks into
// the default user's Secret.
func defaultConnectionData(source map[string][]byte, user, password string) (map[string][]byte, error) {
	ownerPassword := string(source["password"])
	out := make(map[string][]byte, len(source))
	for key, value := range source {
		switch key {
		case "username", "user":
			out[key] = []byte(user)
		case "password":
			out[key] = []byte(password)
		case "uri", "fqdn-uri":
			u, err := url.Parse(string(value))
			if err != nil {
				return nil, fmt.Errorf("parse %s: %w", key, err)
			}
			u.User = url.UserPassword(user, password)
			out[key] = []byte(u.String())
		case "jdbc-uri", "fqdn-jdbc-uri":
			u, err := url.Parse(strings.TrimPrefix(string(value), "jdbc:"))
			if err != nil {
				return nil, fmt.Errorf("parse %s: %w", key, err)
			}
			q := u.Query()
			q.Set("user", user)
			q.Set("password", password)
			u.RawQuery = q.Encode()
			out[key] = []byte("jdbc:" + u.String())
		case "pgpass":
			fields := strings.Split(strings.TrimRight(string(value), "\n"), ":")
			if len(fields) != 5 {
				return nil, fmt.Errorf("pgpass has %d fields, want 5", len(fields))
			}
			fields[3], fields[4] = user, password
			out[key] = []byte(strings.Join(fields, ":") + "\n")
		default:
			if ownerPassword != "" && strings.Contains(string(value), ownerPassword) {
				continue
			}
			out[key] = value
		}
	}
	return out, nil
}
