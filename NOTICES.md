# Third-Party Software Notices and Licenses

This file contains legal notices, copyright statements, and license texts for third-party open-source software utilized by, packaged with, or orchestrated by **Agent Sandbox** (developed by **Boggy Creek Software LLC**).

---

## Table of Third-Party Components

| Component | Upstream Project / Author | License (SPDX) | Usage Context |
| :--- | :--- | :--- | :--- |
| **Go Standard Library** | The Go Authors | [BSD-3-Clause](https://spdx.org/licenses/BSD-3-Clause.html) | Host CLI & Core Library Runtime |
| **Podman** | Red Hat, Inc. & Containers Community | [Apache-2.0](https://spdx.org/licenses/Apache-2.0.html) | Rootless Container Runtime |
| **Conmon** | Red Hat, Inc. & Containers Community | [Apache-2.0](https://spdx.org/licenses/Apache-2.0.html) | OCI Container Monitor |
| **Valkey** | Linux Foundation & Valkey Contributors | [BSD-3-Clause](https://spdx.org/licenses/BSD-3-Clause.html) | Shared In-Memory Streaming Bus |
| **Gitea** | The Gitea Authors | [MIT](https://spdx.org/licenses/MIT.html) | Local Git Forge & REST API Server |
| **PostgreSQL** | PostgreSQL Global Development Group | [PostgreSQL](https://spdx.org/licenses/PostgreSQL.html) | Shared Relational Database Backend |
| **SonarQube Community Build** | SonarSource S.A. | [LGPL-3.0-only](https://spdx.org/licenses/LGPL-3.0-only.html) | Automated Code Quality Inspection |
| **Beads (`bd`)** | Gas Town Hall / Steve Yegge | [MIT](https://spdx.org/licenses/MIT.html) | Graph-Based Task Backlog Tracker |
| **Dolt** | DoltHub, Inc. | [Apache-2.0](https://spdx.org/licenses/Apache-2.0.html) | Version-Controlled Database Engine |
| **tmux** | Nicholas Marriott | [ISC](https://spdx.org/licenses/ISC.html) | In-Container Terminal Multiplexer |
| **OpenSSH** | The OpenBSD Project | [BSD-2-Clause / MIT](https://spdx.org/licenses/BSD-2-Clause.html) | In-Container Unprivileged SSH Daemon |
| **Debian GNU/Linux** | Software in the Public Interest, Inc. | [DFSG / Various](https://www.debian.org/legal/licenses/) | Base OCI Container Operating System |

---

## Third-Party License Texts

### 1. Go Standard Library
**Copyright (c) 2009 The Go Authors. All rights reserved.**

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are
met:

   * Redistributions of source code must retain the above copyright
notice, this list of conditions and the following disclaimer.
   * Redistributions in binary form must reproduce the above
copyright notice, this list of conditions and the following disclaimer
in the documentation and/or other materials provided with the
distribution.
   * Neither the name of Google LLC nor the names of its
contributors may be used to endorse or promote products derived from
this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
"AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

---

### 2. Valkey (BSD 3-Clause)
**Copyright (c) 2024, Valkey contributors. All rights reserved.**  
**Copyright (c) 2006-2024, Salvatore Sanfilippo, Pieter Noordhuis, Matt Stancliff, et al.**

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

* Redistributions of source code must retain the above copyright notice, this
  list of conditions and the following disclaimer.

* Redistributions in binary form must reproduce the above copyright notice,
  this list of conditions and the following disclaimer in the documentation
  and/or other materials provided with the distribution.

* Neither the name of the copyright holder nor the names of its
  contributors may be used to endorse or promote products derived from
  this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE
FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER
CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,
OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

---

### 3. Gitea & Beads (MIT License)
**Copyright (c) 2016-2026 The Gitea Authors**  
**Copyright (c) 2025-2026 Gas Town Hall**

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

---

### 4. Apache License, Version 2.0 (Podman, Conmon, Dolt)
**Copyright (c) Red Hat, Inc., Containers Organization, DoltHub, Inc.**

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

---

### 5. PostgreSQL Database Management System
**Portions Copyright (c) 1996-2026, The PostgreSQL Global Development Group**  
**Portions Copyright (c) 1994, The Regents of the University of California**

Permission to use, copy, modify, and distribute this software and its documentation
for any purpose, without fee, and without a written agreement is hereby granted,
provided that the above copyright notice and this paragraph and the following two
paragraphs appear in all copies.

IN NO EVENT SHALL THE UNIVERSITY OF CALIFORNIA BE LIABLE TO ANY PARTY FOR DIRECT,
INDIRECT, SPECIAL, INCIDENTAL, OR CONSEQUENTIAL DAMAGES, INCLUDING LOST PROFITS,
ARISING OUT OF THE USE OF THIS SOFTWARE AND ITS DOCUMENTATION, EVEN IF THE
UNIVERSITY OF CALIFORNIA HAS BEEN ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

THE UNIVERSITY OF CALIFORNIA SPECIFICALLY DISCLAIMS ANY WARRANTIES, INCLUDING,
BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A
PARTICULAR PURPOSE. THE SOFTWARE PROVIDED HEREUNDER IS ON AN "AS IS" BASIS, AND
THE UNIVERSITY OF CALIFORNIA HAS NO OBLIGATIONS TO PROVIDE MAINTENANCE, SUPPORT,
UPDATES, ENHANCEMENTS, OR MODIFICATIONS.

---

### 6. tmux (ISC License)
**Copyright (c) 2007-2026 Nicholas Marriott <nicholas.marriott@gmail.com>**

Permission to use, copy, modify, and distribute this software for any
purpose with or without fee is hereby granted, provided that the above
copyright notice and this permission notice appear in all copies.

THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
