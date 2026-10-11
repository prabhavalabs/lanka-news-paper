import { Link } from 'react-router'

import { OrnamentRule } from '@/components/newsprint/ornament'
import { PageHead, useDocumentTitle } from '@/components/newsprint/page-head'

export function StaticPage({ title, body, children }: { title: string; body?: string; children?: React.ReactNode }) {
  useDocumentTitle(title)
  return (
    <article className="mx-auto max-w-[44rem]">
      <PageHead title={title} />
      <div className="np-body mt-6 text-[1.0625rem] md:text-[1.125rem]">
        {body ? <p>{body}</p> : null}
        {children}
      </div>
      <OrnamentRule className="mt-10" />
    </article>
  )
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="mt-8">
      <h2 className="np-section-head">{title}</h2>
      <div className="mt-2 space-y-3">{children}</div>
    </section>
  )
}

export function AboutPage() {
  return (
    <StaticPage title="අප ගැන">
      <p className="font-headline text-[1.25rem] leading-[1.7] font-medium">
        තුලාව යනු ශ්‍රී ලංකාවේ මාධ්‍ය ආයතන පළ කරන සිංහල පුවත් එක තැනකට ගෙන එන, කතුවරයෙකු විසින් සකසන පුවත්පතකි.
      </p>
      <Section title="පුවත් පැමිණෙන්නේ කොහෙන්ද?">
        <p>
          අප පුවත් ලියන්නේ නැත. සෑම වාර්තාවක්ම එය පළ කළ මාධ්‍ය ආයතනයට අයත් ය. සෑම පුවතක් යටතේම එය වාර්තා කළ සියලු මාධ්‍ය
          ආයතනවල නම් සහ ඔවුන්ගේ මුල් ලිපියට සබැඳියක් (“මුල් ලිපිය ↗”) ඇත. සම්පූර්ණ ලිපිය කියවීමට මුල් ප්‍රකාශකයා වෙත යන්න.
        </p>
        <p>
          <Link to="/sources" className="np-link">
            අප අනුගමනය කරන මාධ්‍ය ලැයිස්තුව
          </Link>
        </p>
      </Section>
      <Section title="මුල් පිටුව තෝරන්නේ කවුද?">
        <p>
          මුල් පිටුවේ ප්‍රධාන පුවත සහ ප්‍රධාන පුවත් තෝරන්නේ ප්‍රධාන කතුවරයා ය. කතුවරයා තෝරා නැති ඉඩ, මාධ්‍ය කීයක් වාර්තා
          කළාද සහ කොතරම් අලුත්ද යන්න අනුව ස්වයංක්‍රීයව පිරේ. කතුවරයා තෝරාගත් පුවත් කුඩා සලකුණකින් දැක්වේ.
        </p>
      </Section>
      <Section title="එකම පුවත, මාධ්‍ය කිහිපයකින්">
        <p>
          එකම සිදුවීම මාධ්‍ය කිහිපයක් වාර්තා කළ විට අපි ඒවා එක පුවතක් ලෙස එකතු කර, එක් එක් ආයතනයේ වාර්තාව වෙන් වෙන්ව පෙන්වමු.
          සාරාංශ සහ “වාර්තාකරණ රාමුව” පිළිබඳ විශ්ලේෂණ ස්වයංක්‍රීයව සකස් වන අතර ඒවා එලෙසම සලකුණු කර ඇත. ඒවා මාධ්‍ය ආයතනයකට
          දෙන ස්ථිර ලේබල නොවේ.
        </p>
      </Section>
      <Section title="කතුවැකිය">
        <p>
          කතුවැකිය ප්‍රධාන කතුවරයාගේ මතයයි. එය පුවත් වාර්තාවක් නොවන බව සෑම විටම පැහැදිලිව සලකුණු කර ඇත.{' '}
          <Link to="/editorial" className="np-link">
            කතුවැකි කියවන්න
          </Link>
        </p>
      </Section>
      <Section title="නිවැරදි කිරීම්">
        <p>
          වැරැද්දක් හෝ ගැටලුවක් දුටුවොත් එම ලිපියේ “ගැටලුවක් වාර්තා කරන්න” බොත්තම භාවිතා කරන්න.{' '}
          <Link to="/corrections" className="np-link">
            නිවැරදි කිරීමේ ප්‍රතිපත්තිය
          </Link>
        </p>
      </Section>
    </StaticPage>
  )
}
