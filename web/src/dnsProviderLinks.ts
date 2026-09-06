// Official provider sites from the pinned lego catalog; credential links checked
// against Lucky's certificate form. Keep these separate from adapter documentation.
const officialSites: Record<string, string> = {
 oraclecloud: 'https://cloud.oracle.com/home', ovh: 'https://www.ovh.com/',
 ucloud: 'https://www.ucloud.cn/', route53: 'https://aws.amazon.com/route53/',
 freemyip: 'https://freemyip.com/', xinnet: 'https://www.xinnet.com/',
 spaceship: 'https://www.spaceship.com/', namedotcom: 'https://www.name.com/',
 netlify: 'https://www.netlify.com/', rainyun: 'https://www.rainyun.com/',
 digitalocean: 'https://www.digitalocean.com/docs/networking/dns/',
 namesilo: 'https://www.namesilo.com/', godaddy: 'https://godaddy.com/',
 com35: 'https://www.35.cn/', cloudflare: 'https://www.cloudflare.com/dns/',
 desec: 'https://desec.io/', duckdns: 'https://www.duckdns.org/',
 westcn: 'https://www.west.cn/', gcloud: 'https://cloud.google.com/',
 todaynic: 'https://www.todaynic.com/', vultr: 'https://www.vultr.com/',
 dns51: 'https://www.51dns.com/', namecheap: 'https://www.namecheap.com/',
 hetzner: 'https://hetzner.com/', linode: 'https://www.linode.com/',
 vercel: 'https://vercel.com/', porkbun: 'https://porkbun.com/',
 dynadot: 'https://www.dynadot.com/', azuredns: 'https://azure.microsoft.com/services/dns/',
 gandiv5: 'https://www.gandi.net/',
};
const credentialLinks: Record<string, { label: string; url: string }> = {
 alidns: { label: '创建 AccessKey', url: 'https://ram.console.aliyun.com/manage/ak' },
 aliesa: { label: '创建 AccessKey', url: 'https://ram.console.aliyun.com/manage/ak' },
 baiducloud: { label: '创建 AccessKey', url: 'https://console.bce.baidu.com/iam/#/iam/accesslist' },
 tencentcloud: { label: '创建 API 密钥', url: 'https://console.cloud.tencent.com/cam/capi' },
 edgeone: { label: '创建 API 密钥', url: 'https://console.cloud.tencent.com/cam/capi' },
 dnsla: { label: '管理 API 密钥', url: 'https://console.dns.la/account-msg' },
 huaweicloud: { label: '新增访问密钥', url: 'https://console.huaweicloud.com/iam/?locale=zh-cn#/mine/accessKey' },
 jdcloud: { label: '新增访问密钥', url: 'https://uc.jdcloud.com/account/accesskey' },
 volcengine: { label: '新建密钥', url: 'https://console.volcengine.com/iam/keymanage/' },
};
export function dnsProviderLink(code: string, name: string) {
 return credentialLinks[code] ?? (officialSites[code] ? { label: `${name} 官网`, url: officialSites[code] } : undefined);
}
