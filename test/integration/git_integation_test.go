// Copyright (c) 2019-2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: BSD-2-Clause

package integration_test

import (
	"github.com/vmware-tanzu/dependency-labeler/pkg/metadata"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("deplab git", func() {
	Context("when I supply git repo(s) as argument(s)", func() {
		var (
			metadataLabel       metadata.Metadata
			gitDependencies     []metadata.Dependency
			additionalArguments []string
		)

		JustBeforeEach(func() {
			metadataLabel = runDeplabAgainstTar(getTestAssetPath("image-archives/tiny.tgz"), additionalArguments...)
			gitDependencies = selectGitDependencies(metadataLabel.Dependencies)
		})

		Context("when I supply only one --git argument", func() {
			BeforeEach(func() {
				additionalArguments = []string{}
			})

			It("adds a git dependency", func() {
				Expect(gitDependencies).To(HaveLen(1))

				gitDependency := gitDependencies[0]
				Expect(gitDependency.Type).ToNot(BeEmpty())

				By("adding the git commit of HEAD to a git dependency")
				Expect(gitDependency.Type).To(Equal("package"))
				Expect(gitDependency.Source).To(Not(BeNil()))
				Expect(gitDependency.Source.Version["commit"]).To(Equal(commitHash))

				By("providing a dependency metadata object")
				Expect(gitDependency.Source.Metadata).To(Not(BeNil()))
				gitSourceMetadata := gitDependency.Source.Metadata.(map[string]interface{})

				By("adding the git remote to a git dependency")
				Expect(gitSourceMetadata["url"].(string)).To(Equal("https://example.com/example.git"))

				By("adding refs for the current HEAD")
				Expect(gitSourceMetadata["refs"].([]interface{})).To(HaveLen(1))
				Expect(gitSourceMetadata["refs"].([]interface{})[0].(string)).To(Equal("bar"))

				By("not adding refs that are not the current HEAD")
				Expect(gitSourceMetadata["refs"].([]interface{})[0].(string)).ToNot(Equal("foo"))
			})
		})

		Context("when I supply multiple git repositories as separate arguments", func() {
			BeforeEach(func() {
				additionalArguments = []string{"--git", pathToGitRepo}
			})

			It("adds multiple gitDependencies entries", func() {
				Expect(gitDependencies).To(HaveLen(2))
			})
		})
	})

	Context("when I supply non-git repo as an argument", func() {
		It("exits with an error message", func() {
			By("executing it")
			_, stdErr := runDepLab([]string{
				"--image-tar", getTestAssetPath("image-archives/tiny.tgz"),
				"--git", "/dev/null",
				"--metadata-file", "doesnotmatter6",
			}, 1)

			Expect(string(getContentsOfReader(stdErr))).To(ContainSubstring("cannot open git repository \"/dev/null\""))
		})
	})
})

func selectGitDependencies(dependencies []metadata.Dependency) []metadata.Dependency {
	var gitDependencies []metadata.Dependency
	for _, dependency := range dependencies {
		if dependency.Source.Type == metadata.GitSourceType {
			gitDependencies = append(gitDependencies, dependency)
		}
	}
	return gitDependencies
}
