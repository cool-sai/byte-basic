"""python3 -m unittest -v（在本目录下运行）"""
import os
import tempfile
import unittest

import renew


class TestParse(unittest.TestCase):
    def test_parse_domains(self):
        self.assertEqual(renew.parse_domains(""), [])
        self.assertEqual(renew.parse_domains("news.lssl.work"), [["news.lssl.work"]])
        self.assertEqual(renew.parse_domains(" News.lssl.work ; lssl.work, www.lssl.work ;; "),
                         [["news.lssl.work"], ["lssl.work", "www.lssl.work"]])


class TestWebrootNames(unittest.TestCase):
    def test_only_webroot(self):
        le = tempfile.mkdtemp()
        os.makedirs(os.path.join(le, "renewal"))
        confs = {
            "news.lssl.work": "[renewalparams]\nauthenticator = webroot\nwebroot_path = /acme,\n",
            "lssl.work": "[renewalparams]\nauthenticator = manual\npref_challs = dns-01,\n",
            "img.lssl.work": "[renewalparams]\nauthenticator = manual\n",
        }
        for name, body in confs.items():
            with open(os.path.join(le, "renewal", name + ".conf"), "w") as f:
                f.write(body)
        self.assertEqual(renew.webroot_cert_names(le), ["news.lssl.work"])


if __name__ == "__main__":
    unittest.main()
