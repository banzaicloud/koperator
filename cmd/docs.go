// Copyright 2020 Banzai Cloud
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"path/filepath"

	"emperror.dev/errors"
	"github.com/MakeNowJust/heredoc"
	"github.com/banzaicloud/operator-tools/pkg/docgen"
	"github.com/banzaicloud/operator-tools/pkg/utils"
)

var logger = utils.Log

func main() {
	crds()
}

func crds() {
	lister := docgen.NewSourceLister(
		map[string]docgen.SourceDir{
			"kafka": {Path: "pkg/sdk/v1beta1", DestPath: "docs/types"},
		},
		logger.WithName("lister"))

	lister.IgnoredSources = []string{
		"null",
		".*.deepcopy",
		".*_test",
		".*_info",
	}

	lister.Index = docgen.NewDoc(docgen.DocItem{
		Name:     "_index",
		DestPath: "docs/types",
	}, logger.WithName("typedoc"))

	lister.Header = heredoc.Doc(`
		---
		title: CRDs
		weight: 1000
		generated_file: true
		---
		
		The following types are available. For details, click on the name of a type. 
		`,
	)

	lister.Footer = heredoc.Doc(`

	`)

	lister.DocGeneratedHook = func(document *docgen.Doc) error {
		relPath, err := filepath.Rel(lister.Index.Item.DestPath, document.Item.DestPath)
		if err != nil {
			return errors.WrapIff(err, "failed to determine relpath for %s", document.Item.DestPath)
		}
		lister.Index.Append(fmt.Sprintf("- **[%s](%s)** %s",
			document.Item.Name,
			filepath.Join(relPath, document.Item.Name+".md"),
			document.Desc))
		return nil
	}

	if err := lister.Generate(); err != nil {
		panic(err)
	}
}
