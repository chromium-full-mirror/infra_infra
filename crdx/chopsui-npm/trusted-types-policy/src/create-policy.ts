// Copyright 2022 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

import * as DOMPurify from 'dompurify';

// Required import because we have 2 modules with the same name
import { trustedTypes } from 'trusted-types';

export default function createInnerHTMLSanitizingPolicy() {
  if (!window.trustedTypes || !window.trustedTypes.createPolicy) {
    window.trustedTypes = trustedTypes;
  }

  window.trustedTypes!.createPolicy('default', {
    createHTML: (string) => DOMPurify.sanitize(string, {
      // createHTML expects a "string" return type rather than
      // a TrustedType object return type.
      RETURN_TRUSTED_TYPE: false,
      FORBID_TAGS: ['style'],
    }),
  });
}
