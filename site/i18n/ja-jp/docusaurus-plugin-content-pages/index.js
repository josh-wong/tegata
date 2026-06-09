import clsx from 'clsx';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';
import styles from '@site/src/pages/index.module.css';
import featureStyles from '@site/src/components/HomepageFeatures/styles.module.css';

const FeatureList = [
  {
    title: 'クレデンシャルをどこへでも持ち運ぶ',
    description: (
      <>
        暗号化されたボールトは標準の USB ドライブや microSD カードに保存されます。クラウドアカウントも同期サービスも不要です。クレデンシャルはポータブルで、どこでもすぐに使えます。
      </>
    ),
  },
  {
    title: 'あなたの鍵、あなたがコントロール',
    description: (
      <>
        クレデンシャルは AES-256-GCM とあなたが選択したパスフレーズで暗号化されます。すべての認証はデバイス上でローカルに実行されます。あなたが許可しない限り、データはハードウェアの外に出ません。
      </>
    ),
  },
  {
    title: '改ざん防止の監査ログ',
    description: (
      <>
        すべての認証イベントは ScalarDL Ledger によるハッシュチェーン監査ログに記録されます。<code>tegata verify</code> をいつでも実行して、ログが改ざんされていないことを確認できます。
      </>
    ),
  },
];

function Feature({title, description}) {
  return (
    <div className={clsx('col col--4')}>
      <div className="text--center padding-horiz--md">
        <Heading as="h3">{title}</Heading>
        <p>{description}</p>
      </div>
    </div>
  );
}

function HomepageFeatures() {
  return (
    <section className={featureStyles.features}>
      <div className="container">
        <div className="row">
          {FeatureList.map((props, idx) => (
            <Feature key={idx} {...props} />
          ))}
        </div>
      </div>
    </section>
  );
}

function HomepageHeader() {
  const {siteConfig} = useDocusaurusContext();
  return (
    <header className={clsx('hero hero--primary', styles.heroBanner)}>
      <div className="container">
        <Heading as="h1" className="hero__title">
          {siteConfig.title}
        </Heading>
        <p className="hero__subtitle">認証履歴をどこへでも。整合性を確認。</p>
        <div className={styles.buttons}>
          <Link
            className="button button--secondary button--lg"
            to="/docs/introduction">
            はじめる
          </Link>
        </div>
      </div>
    </header>
  );
}

export default function Home() {
  const {siteConfig} = useDocusaurusContext();
  return (
    <Layout
      title={siteConfig.title}
      description="ポータブルな認証情報ストレージと改ざん防止の監査ログ">
      <HomepageHeader />
      <main>
        <HomepageFeatures />
      </main>
    </Layout>
  );
}
