// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"

	"github.com/hashicorp/go-getter"
	"github.com/hashicorp/go-multierror"
	"github.com/kubernetes-sigs/bootkube/pkg/tlsutil"
	"github.com/talos-systems/bootkube-plugin/pkg/asset"

	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/constants"
	tnet "github.com/talos-systems/talos/pkg/net"
)

// nolint: gocyclo
func generateAssets(config runtime.Configurator) (err error) {
	if err = os.MkdirAll(constants.ManifestsDirectory, 0644); err != nil {
		return err
	}

	// Ensure assets directory does not exist / is left over from a failed install
	if err = os.RemoveAll(constants.AssetsDirectory); err != nil {
		// Ignore if the directory does not exist
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}

	peerCrt, err := ioutil.ReadFile(constants.KubernetesEtcdPeerCert)
	if err != nil {
		return err
	}

	block, _ := pem.Decode(peerCrt)
	if block == nil {
		return errors.New("failed to decode peer certificate")
	}

	peer, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse client certificate: %w", err)
	}

	caCrt, err := ioutil.ReadFile(constants.KubernetesEtcdCACert)
	if err != nil {
		return err
	}

	block, _ = pem.Decode(caCrt)
	if block == nil {
		return errors.New("failed to decode CA certificate")
	}

	ca, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse etcd CA certificate: %w", err)
	}

	peerKey, err := ioutil.ReadFile(constants.KubernetesEtcdPeerKey)
	if err != nil {
		return err
	}

	block, _ = pem.Decode(peerKey)
	if block == nil {
		return errors.New("failed to peer key")
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse client key: %w", err)
	}

	etcdServer, err := url.Parse("https://127.0.0.1:2379")
	if err != nil {
		return err
	}

	_, podCIDR, err := net.ParseCIDR(config.Cluster().Network().PodCIDR())
	if err != nil {
		return err
	}

	_, serviceCIDR, err := net.ParseCIDR(config.Cluster().Network().ServiceCIDR())
	if err != nil {
		return err
	}

	urls := []string{config.Cluster().Endpoint().Hostname()}
	urls = append(urls, config.Cluster().CertSANs()...)
	altNames := altNamesFromURLs(urls)

	k8sCA, err := config.Cluster().CA().GetCert()
	if err != nil {
		return fmt.Errorf("failed to get Kubernetes CA certificate: %w", err)
	}

	k8sKey, err := config.Cluster().CA().GetRSAKey()
	if err != nil {
		return fmt.Errorf("failed to parse Kubernetes key: %w", err)
	}

	apiServiceIP, err := tnet.NthIPInNetwork(serviceCIDR, 1)
	if err != nil {
		return err
	}

	dnsServiceIP, err := tnet.NthIPInNetwork(serviceCIDR, 10)
	if err != nil {
		return err
	}

	images := asset.DefaultImages

	images.Hyperkube = config.Machine().Kubelet().Image()

	// Allow for overriding by users via config data
	images.CoreDNS = config.Cluster().CoreDNS().Image()
	images.PodCheckpointer = config.Cluster().PodCheckpointer().Image()

	conf := asset.Config{
		ClusterName:                config.Cluster().Name(),
		APIServerExtraArgs:         config.Cluster().APIServer().ExtraArgs(),
		ControllerManagerExtraArgs: config.Cluster().ControllerManager().ExtraArgs(),
		SchedulerExtraArgs:         config.Cluster().Scheduler().ExtraArgs(),
		CACert:                     k8sCA,
		CAPrivKey:                  k8sKey,
		EtcdCACert:                 ca,
		EtcdClientCert:             peer,
		EtcdClientKey:              key,
		EtcdServers:                []*url.URL{etcdServer},
		EtcdUseTLS:                 true,
		ControlPlaneEndpoint:       config.Cluster().Endpoint(),
		LocalAPIServerPort:         config.Cluster().LocalAPIServerPort(),
		APIServiceIP:               apiServiceIP,
		DNSServiceIP:               dnsServiceIP,
		PodCIDR:                    podCIDR,
		ServiceCIDR:                serviceCIDR,
		NetworkProvider:            config.Cluster().Network().CNI().Name(),
		AltNames:                   altNames,
		Images:                     images,
		BootstrapSecretsSubdir:     "/assets/tls",
		BootstrapTokenID:           config.Cluster().Token().ID(),
		BootstrapTokenSecret:       config.Cluster().Token().Secret(),
		AESCBCEncryptionSecret:     config.Cluster().AESCBCEncryptionSecret(),
		ClusterDomain:              config.Cluster().Network().DNSDomain(),
	}

	if err = asset.Render(constants.AssetsDirectory, conf); err != nil {
		return err
	}

	// If "custom" is the CNI, we expect the user to supply one or more urls that point to CNI yamls
	if config.Cluster().Network().CNI().Name() == constants.CustomCNI {
		if err = fetchManifests(config.Cluster().Network().CNI().URLs(), map[string]string{}); err != nil {
			return err
		}
	}

	if len(config.Cluster().ExtraManifestURLs()) > 0 {
		if err = fetchManifests(config.Cluster().ExtraManifestURLs(), config.Cluster().ExtraManifestHeaderMap()); err != nil {
			return err
		}
	}

	return nil
}

func altNamesFromURLs(urls []string) *tlsutil.AltNames {
	var an tlsutil.AltNames

	for _, u := range urls {
		ip := net.ParseIP(u)
		if ip != nil {
			an.IPs = append(an.IPs, ip)
			continue
		}

		an.DNSNames = append(an.DNSNames, u)
	}

	return &an
}

// fetchManifests will lay down manifests in the provided urls to the bootkube assets directory
func fetchManifests(urls []string, headers map[string]string) error {
	ctx := context.Background()

	var result *multierror.Error

	for _, url := range urls {
		fileName := path.Base(url)

		pwd, err := os.Getwd()
		if err != nil {
			result = multierror.Append(result, err)
			continue
		}

		// Disable netrc since we don't have getent installed, and most likely
		// never will.
		httpGetter := &getter.HttpGetter{
			Netrc:  false,
			Client: http.DefaultClient,
		}

		httpGetter.Header = make(http.Header)

		for k, v := range headers {
			httpGetter.Header.Add(k, v)
		}

		getter.Getters["http"] = httpGetter
		getter.Getters["https"] = httpGetter

		// We will squirrel all user-supplied manifests into a `zzz-talos` directory.
		// Bootkube applies manifests alphabetically, so pushing these into a subdir with this name
		// allows us to ensure they're the last things that get applied and things like PSPs and whatnot are present
		client := &getter.Client{
			Ctx:     ctx,
			Src:     url,
			Dst:     filepath.Join(constants.AssetsDirectory, "manifests", "zzz-talos", fileName),
			Pwd:     pwd,
			Mode:    getter.ClientModeFile,
			Options: []getter.ClientOption{},
		}

		if err = client.Get(); err != nil {
			result = multierror.Append(result, err)
			continue
		}
	}

	return result.ErrorOrNil()
}
