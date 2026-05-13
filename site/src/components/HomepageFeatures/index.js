import clsx from 'clsx';
import Heading from '@theme/Heading';
import styles from './styles.module.css';

const FeatureList = [
  {
    title: 'Carry your credentials anywhere',
    // Svg: require('@site/static/img/undraw_docusaurus_mountain.svg').default,
    description: (
      <>
        Your encrypted vault lives on a standard USB drive or microSD card. No cloud accounts, no sync services—just your credentials, portable and ready wherever you are.
      </>
    ),
  },
  {
    title: 'Your keys, your control',
    // Svg: require('@site/static/img/undraw_docusaurus_tree.svg').default,
    description: (
      <>
        Credentials are encrypted with AES-256-GCM and a passphrase you choose. All authentication happens locally on your device. Nothing leaves your hardware unless you decide it does.
      </>
    ),
  },
  {
    title: 'Tamper-evident audit logging',
    // Svg: require('@site/static/img/undraw_docusaurus_react.svg').default,
    description: (
      <>
        Every authentication event is recorded in a hash-chained audit log backed by ScalarDL Ledger. Run <code>tegata verify</code> at any time to confirm the log has not been altered.
      </>
    ),
  },
];

function Feature({Svg, title, description}) {
  return (
    <div className={clsx('col col--4')}>
      {/* <div className="text--center">
        <Svg className={styles.featureSvg} role="img" />
      </div> */}
      <div className="text--center padding-horiz--md">
        <Heading as="h3">{title}</Heading>
        <p>{description}</p>
      </div>
    </div>
  );
}

export default function HomepageFeatures() {
  return (
    <section className={styles.features}>
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
