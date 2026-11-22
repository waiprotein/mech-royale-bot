package mech

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
)

const lat = 0.075

type Track struct {
	MechSeen
	Seen    float64
	HitMe   float64
	Dist    float64
	LOS     bool
	Closing float64
	Engage  bool
	// CalmAt (hunthide): last time this mech was seen within 200 m with its aim more than 0.6 rad off us;
	// reset to -100 when it is seen aiming at us (within 0.22 rad, giddy's "aimed" test).
	CalmAt float64
	// HurtAt (tpjoin): last time this mech lost more than 20 HP between two sightings <= 0.5 s apart with no hit
	// of ours in between (someone else is hurting it). TPAt / OppDist (tpjoin): last time the third-party join
	// rule held for it, and the distance from it to the opponent it was fighting then.
	HurtAt, TPAt, OppDist float64
	// LOSAt (precharge): last time our muzzle had a clear line to it.
	LOSAt float64
	// LRAt (lrlaser): last time the long-range laser opener rule held for it.
	LRAt float64
	// LaserAt (lcdpunish): last time we saw (beam) or heard (laser sound on its bearing) this mech fire its laser.
	LaserAt float64
}

type known struct {
	ID   int
	Kind string
	Pos  Vec3
	HP   float64
	Seen float64
}

type circle struct{ X, Y, R float64 }

type Brain struct {
	W    *World
	Cfg  Config
	Slot int
	rng  *rand.Rand
	P    Params

	tracks  map[int]*Track
	npcs    map[int]*Track
	crates  map[int]*known
	cubes   map[int]*known
	pickups map[int]*known
	bad     map[int]float64

	now float64

	path     []int32
	pathGoal [2]float64
	pathAt   float64
	pathI    int

	strafe      float64
	strafeUntil float64

	retreat   bool
	coverAt   float64
	coverGoal [2]float64
	coverOK   bool

	lastHurt float64
	// armboost: when (match s + 1; 0 = never) and where the arm weapon run was last set.
	abAt  float64
	abPos [2]float64
	// rescan: end of the current full spin, and when the last one started.
	rsUntil, rsAt float64
	hurtBearing   float64
	hurtSeenBy    bool

	lootID    int
	lootSince float64

	hold      circle
	holdStart float64

	homeXY       [2]float64
	homeAt       float64
	homeSet      bool
	explore      [2]float64
	exploreUntil float64

	stuckSince   float64
	unstuckUntil float64
	unstuckDir   [2]float64
	lastThreatAt float64
	lastAim      [2]float64
	laserHeld    float64
	Mode         string
	Panics       int

	escLaunch, escLand [2]float64
	escAt              float64
	escOK, escFlying   bool
	oddsFleeUntil      float64
	base               Params
	baseSet, pracOn    bool
	openOn             bool
	npcHurt            float64
	npcBearing         float64
	notPractice        bool
	startAlive, kills  int
	otherDead          int
	stickID            int
	stickAt, loseAt    float64
	myHitAt            map[int]float64
	ffaNow             bool
	maxUnexpl          int
	dropOKAt           float64 // ffadrops: last time dropsOK passed our loot-target drop (commitment)
	dropOrder          []int   // ffadrops: supply drop ids in the order they were first listed (= call order)
	safeMode           bool
	exch               map[int]*exchange
	showCfg            bool
	// facesound: time (match s) and bearing of the newest unexplained weapon sound.
	soundAt, soundBearing float64
	// backopen: when we first held our opener on each mech id.
	boWait map[int]float64
	// peekcd (bit 2): id of the hidden foe whose laser we heard fire, and when its cooldown ends.
	cdID    int
	cdUntil float64
	// laserlos: when each mech was first seen charging a laser aimed at us (this charge); entries are dropped when it
	// stops charging, aims away, fires (beam seen) or the charge must have auto-released.
	lzSince map[int]float64
	// threatfocus: per attacker id, decayed damage it dealt us and when that was last updated.
	thrDmg map[int][2]float64
	// cornercover: coverMove's search budget (candidate checks, path steps of 2 m); 0 = giddy's 350 / 50.
	coverChecks, coverSteps int
	ccFailAt                float64
	// kitew: fightMove adds KiteW per metre of distance gained from the target while this is set (fight branch only).
	kiteOn bool
	// regenlock: per mech id, the damage we landed since its regen window last (re)started and our last hit on it.
	rlWin map[int]*regenWin
	// regenhide: latched while regenerating in cover, and when the latch started.
	regenOn bool
	regenAt float64
	// homelos: eyes of the recently seen mechs for this tick's home costs (valid while losTick == tick+1), and
	// whether a fresh mech saw our home at the previous check.
	losEyes []Vec3
	losTick int
	homeVis bool
	// sqret: squad-mode HP retreat latch. finMove (sqband): set while fightMove runs for the finish rule.
	sqRetOn bool
	finMove bool
	// sqcorner: when the current sqret latch started.
	sqRetAt float64
}

type Params struct {
	RetreatHP   float64
	RecoverHP   float64
	GunRange    float64
	LightRange  float64
	LaserRange  float64
	EngageDist  float64
	FarmNPC     bool
	Explore     bool
	Ghost       bool
	HomeExpo    float64
	LootRadius  float64
	CrateUntil  float64
	UseDrops    bool
	Dodge       bool
	NoMortar    bool
	Peek        bool
	LaserFirst  bool
	CrowdHide   bool
	Vulture     float64
	Ambush      float64
	LaserSeek   float64
	Lethal      bool
	Alert       float64
	Shy         float64
	EarlyHide   float64
	Evade       bool
	Odds        float64
	OddsFlee    float64
	Practice    bool
	Stick       float64
	HPw         float64
	LowBonus    float64
	MaxTarget   float64
	LoseRule    bool
	FightLoot   float64
	FarmHP      float64
	AllFoes     bool
	CubeGoal    float64
	Econ        bool
	LaserUp     bool
	EconSafe    float64
	Opening     bool
	NanoUp      bool
	Finish      float64
	FinishHP    float64
	FinishRush  bool
	Pursue      float64
	FightBoost  float64
	DodgeBoost  float64
	ThreatBoost float64
	SpeedUp     bool
	Backstab    float64
	LethalK     float64
	// Our additions (FFA end game). LateR: next-circle radius at or below which the bot stops retreating
	// (giddy: 70). HoldOut: match time until which, once the next circle is the final one (<= 30 m), a ghost
	// waits at a hidden spot at least HoldMin m from the final centre instead of walking into the pile-up.
	LateR   float64
	HoldOut float64
	HoldMin float64
	// FFAMin: deaths we did not cause needed before the bot decides it is not in practice (giddy: 1).
	FFAMin float64
	// SqLaserRange / SqFinish: practice-mode overrides for local tests (0 = giddy's 120 / 26).
	SqLaserRange float64
	SqFinish     float64
	// PracAt: match time after which, with no unexplained death yet, practice (squad) params switch on (giddy: 20).
	PracAt float64
	// BeamFFA: treat a laser/lightning shot from one mech landing on another mech (not us) that lost HP as proof
	// of a free-for-all (the practice squad never fights itself), and leave practice mode at once.
	BeamFFA bool

	// FFA experiments for the local sim. Every one is off by default (DefaultParams plays exactly as before);
	// the MECH_FLAGS key is in brackets.
	//
	// OpenUntil [openuntil]: match time after which giddy's Opening switch [opening] turns off, even if no
	// death we did not cause has happened yet (0 = no time cap, giddy's behaviour).
	OpenUntil float64
	// GunOnlyOut [gunonlyout]: while we hold only the gun, any visible mech holding a laser or lightning makes
	// us `outgunned` at any range and HP (giddy: only a laser target beyond 55 m with us below 80% HP), which
	// sends us down the existing swamped -> retreat -> cover path. Not mode-gated, but inert under squad params
	// because their LoseRule overwrites the retreat flag every tick.
	GunOnlyOut bool
	// LateAlive [lateal]: the `alive <= N` half of `late` (no retreat, no shy/evade/hurt-cover in the end game).
	// giddy: 3. 0 disables it, so retreat stays allowed with 3 or fewer alive (the LateR half still applies).
	// squadParams resets it to 3, so practice play is unchanged.
	LateAlive int
	// LightLOS [lightlos]: FFA-only (gated on !pracOn: base params, i.e. before PracAt or after the FFA latch).
	// When lightning is our only second weapon and the foe (the target, or the hidden foe we would peek)
	// holds a laser beyond lightning range + 5 m, take the swamped -> retreat -> cover path (break LOS)
	// instead of trading gun fire, unless we are finishing it.
	LightLOS bool
	// DuelFlee [duelflee]: FFA-only (!pracOn). If duelOdds (the duel model refitted on recorded top-16
	// showdowns) for the current target is below this, flee for 5 s through OddsFlee's plumbing
	// (oddsFleeUntil -> swamped -> retreat -> cover), unless we are finishing it. 0 = off.
	DuelFlee float64
	// FaceSound [facesound]: FFA-only (!pracOn). With no target, no engaged/hit-us mech to watch, no recent
	// damage bearing and nothing being farmed, turn the torso (yaw) toward the newest unexplained gun, laser,
	// laser_charge or lightning sound heard "near" or "mid" in the last 1.5 s, so we face (see, and take front
	// armour from) the shooter first. "Unexplained" = no mech we see this tick within 0.35 rad of its bearing.
	FaceSound bool
	// HuntHide [hunthide]: EXPERIMENTAL. Acts only while squad params are on (pracOn: after PracAt, before the
	// first death we did not cause), which in an FFA is the window where every giddy copy hunts. Squad params
	// stay on, but squad engagements are additionally filtered through giddy's FFA ghost engage rule (within
	// EngageDist 60 m, hit us in the last 4 s, charging a laser at us within 120 m, below 350 HP within 120 m,
	// or 2 alive) ONLY IF at least 2 other mechs were seen within 200 m in the last 5 s with their aim not at
	// us (Track.CalmAt). Heuristic: the practice house squad hunts us, FFA copies hunt each other.
	// Risk: idle house-squad members that do not aim at us would also trigger it and change practice play.
	HuntHide bool
	// KeepLaser [keeplaser]: not mode-gated. giddy's LaserFirst (on in both modes) already ranks laser over
	// lightning in wantPickup, so a held laser is never swapped for lightning and a laser is wanted over held
	// lightning. KeepLaser adds what is missing: that ranking even with LaserFirst off, and, while we hold
	// lightning, a laser pickup within loot reach (LootRadius, or LaserSeek if larger) is valued 150 (giddy:
	// 60, below a cube's 75) in chooseLoot, so we go and swap it in. No effect under squad params (LaserSeek
	// 240 already values it 150).
	KeepLaser bool
	// FFAStrict [ffastrict]: our FFA-only features (duelflee, lightlos, facesound, gunonlyout, lateal) act only after
	// giddy's detector has proven a free-for-all (a death we did not cause). Practice play then stays identical to
	// giddy e19e3c8 at salt 0 (6.19 on the board); other changes to practice play regress to ~5.5 (salt tests).
	FFAStrict bool
	// FFAStrictN [ffastrictn]: with FFAStrict, also require at least N deaths we did not cause (default 1). giddy's latch
	// fires falsely in ~1 of 48 practice matches (a house bot dying to the zone or a drone); 2 keeps practice play
	// identical while real FFAs reach 2 such deaths a few seconds later.
	FFAStrictN float64
	// SqUseDrops [squsedrops] / SqHPw [sqhpw]: practice-mode (squad params) only. Capture supply drops against the
	// house squad (team recursive: giddy + drops = 6.16 on builders-4), and the squad target HP weight (giddy
	// 0.18; IT_Professionals 0.21). FFA play is unchanged except in the squad window before the first death.
	SqUseDrops bool
	SqHPw      float64
	// CrowdR [crowdr] / HomeFrac [homefrac]: FFA-only spacing. Crowd penalty radius in homeCost (giddy 80 m) and
	// how far out of the next circle home spots are sampled (giddy 0.7 of its radius). 0 = giddy.
	CrowdR   float64
	HomeFrac float64

	// Opening family (lab/opening), all off by default.
	// Arm [arm]: the main chat's arming recipe (results/opening/FROM_MAIN.md #6). While gun-only and before
	// ArmUntil [armuntil] (default 30 s): any ground laser/lightning within ArmR [armr] m (default 120; ArmRFar
	// [armrfar], default 160, if no mech was tracked within 150 m in the last 5 s) is a loot candidate valued at
	// least ArmV [armv] (default 400), nearest first, unless a mech tracked in the last 5 s is within ArmRoute
	// [armroute] m (default 60) of the straight route; no gun fire at mechs beyond ArmFire [armfire] m (default
	// 45) unless that mech hit us in the last 4 s; with no weapon candidate, ground crates within ArmCrate
	// [armcrate] m (default 60) are valued 150. squadParams turns Arm off unless ArmPrac [armprac] is set
	// (so by default it can change practice play only before PracAt).
	Arm     bool
	ArmPrac bool
	// ArmStrict [armstrict]: arm acts only after giddy's detector has proven a free-for-all (b.notPractice: a
	// death we did not cause), so practice play stays identical (footprint ~0, like FFAStrict).
	ArmStrict bool
	// ArmWin [armwin]: arm acts only while squad params are on (pracOn: from PracAt to the first death we did not
	// cause, the FFA "aggression window") or after a proven free-for-all (notPractice); never in the first PracAt
	// seconds before either. In practice that limits it to PracAt..ArmUntil while still gun-only.
	ArmWin   bool
	ArmUntil float64
	ArmR     float64
	ArmRFar  float64
	ArmV     float64
	ArmRoute float64
	ArmFire  float64
	ArmCrate float64
	// Scan [scan]: for the first Scan s, when nothing else needs the aim, spin the torso at full turn rate so the
	// 120 deg vision wedge sweeps all round and the bot learns the pickups near its spawn.
	Scan float64
	// CubePush [cubepush] / CubePushK [cubepushk]: FFA-only (ffaNow: after giddy's latch), until match time CubePush
	// and below the cube stat cap, cubes and crates are worth CubePushK (default 2.5) times more in chooseLoot
	// (cube 75 -> 188, crate 45 -> 113), to test the real-data marker "cubes at 60 s". 0 = off.
	CubePush  float64
	CubePushK float64
	// ArmFirst [armfirst]: with Arm, ALL modes, before PracAt (20 s) while arm is active (gun-only): weapon first.
	// The laser/lightning reach is always ArmRFar (160 m; arm alone drops to ArmR 120 m when a mech is within
	// 150 m); a candidate's value is divided by 1 + the number of mechs seen in the last 5 s that have line of
	// sight to it (prefer the pickup fewer mechs can see); and while any weapon candidate exists, cube, crate and
	// drone detours are skipped (arm alone still lets a close cube or drone win, and keeps crates at giddy's
	// value). With no candidate, arm's crate fallback still applies. No boost is added. Real data (Winner
	// playbook "First 20 s"): second weapon by 20 s rho +0.22; weapon-before-cubes route +0.20, cube-first -0.11.
	// Also (rival survey SPEC 2 "armrace", mechanics only): a pickup another mech is much closer to (< 0.6 x our
	// distance) is worth x0.3; a pickup the zone reaches before our ETA + 8 s is skipped; and under fire we keep
	// running to a weapon within 90 m when the foe, and every mech we see, is at least 70 m away.
	ArmFirst bool
	// ArmBoost [armboost]: ALL modes, while arm is active (gun-only, Arm on) and before 30 s: boost while walking to
	// the arm weapon pickup (loot or armfirst's run under fire) when it is more than 25 m away and energy > 30.
	// Opening weapons Table 3a: a 90-150 m run is 9.4-12.0 s walking vs 5.8-8.0 s boosting. false = off.
	ArmBoost bool
	// SqJink [sqjink]: SQUAD MODE ONLY (pracOn). fightMove's strafe always reverses, every 0.2-0.5 s, instead of
	// giddy's 60% chance every 0.3-1.0 s. false = off.
	SqJink bool
	// GunOnlyR [gunonlyr]: with GunOnlyOut, only armed mechs closer than this count (0 = any range).
	GunOnlyR float64
	// CfgGate [cfggate]: allow the opening only when hello.config has the showdown/kit schedule (zone 560 m,
	// first hold 60 s + shrink 50 s, drops at 70/140/210 s). If practice rooms use another schedule this is a
	// perfect t=0 practice-vs-FFA test (to be confirmed on the board: the gated build must score exactly 6.19).
	CfgGate bool
	// CfgNoSquad [cfgnosquad]: with CfgGate, under the showdown schedule never switch to squad (practice) params.
	// CfgFFA [cfgffa]: with CfgGate, under the showdown schedule treat the match as a proven FFA from t = 0, so
	// the strict FFA features act at once instead of after the first unexplained death.
	CfgNoSquad bool
	CfgFFA     bool
	// TPJoin [tpjoin] / TPHP [tphp]: FFA-only (ffaOn). Third-party a busy mech: engage a visible mech that is
	// fighting another mech (beam between them, mutual aim, or it is losing HP to someone else while one aims at
	// the other; see tpOpp) and not us, when we are more than TPJoin m farther from it than its opponent, our HP
	// share is >= TPHP (0 = 0.7), its HP <= ours and it is within our reach. prefRange then keeps us that far
	// out, so its nearest-first targeting stays on the opponent. 0 = off; the value is the margin in m.
	TPJoin float64
	TPHP   float64
	// BackOpen [backopen]: FFA-only (ffaOn). On a fresh opener (it has not hit us within 6 s, we have not hit it
	// within 8 s, it is beyond its all-round vision ring + 2 m, not charging, our HP > 60%), hold gun and laser
	// fire until its aim is more than 90° + 0.15 rad off us (back turned and blind to us). Meanwhile keep a full
	// laser charge, never start a charge it can see, and let a held charge go if it turns to see us. At most
	// BackOpen s per target, then fire anyway. Fire returns at once when it hits us. 0 = off.
	BackOpen float64
	// PreCharge [precharge]: FFA-only (ffaOn). Charge or hold the laser for a hidden foe (no target in LOS) only
	// while that foe had muzzle LOS within the last PreCharge s and is within 80 m. Otherwise no charge is
	// started, and a held one is let go at once (its cooldown starts now) instead of being carried to giddy's
	// 2.4 s release into cover. 0 = off (giddy: hold any hidden-foe charge to 2.4 s).
	// PeekCD [peekcd]: FFA-only, bits. 1: while our laser is cooling down, do not step out to peek at a hidden
	// foe. 2: when a laser shot is heard within 0.35 rad of a hidden laser-armed foe's bearing, step out for its
	// 1.6 s cooldown and gun it (it cannot answer with the laser). 3 = both (2 wins when both apply). A visible
	// mech charging at us still sends us back into cover.
	PreCharge float64
	PeekCD    int
	// LRLaser [lrlaser] / LRHP [lrhp] / LRKeep [lrkeep]: FFA-only (ffaOn). TigerTown27-style long-range laser
	// opener (vault: Data/Rival TigerTown27.md, Gap review (Sat 23h).md). Engage a mech at EngageDist..LRLaser m
	// (capped at MaxTarget, since a farther mech cannot be targeted) when: we hold a laser that is ready
	// (cooldown <= 0.3 s), it is in view with LOS, it has not noticed us (aim more than 0.22 rad off us) or it is
	// busy with another mech (tpOpp), it is not charging a laser at us (aim within 0.5 rad), our HP >= LRHP x its
	// HP (0 = 1.0), we are not retreating, and at most 1 other mech is in view within 150 m of us. The rule is held
	// 1.5 s (aim flicker), then for 4 s after each of our hits on it while our HP still beats LRHP x its HP and it
	// is within range (follow-through: once hit it turns to us and giddy's rule alone would drop it). The normal
	// fire code opens with the gun at once and charges the laser in parallel, then releases the full charge on aim
	// (its cap is the weapon's own range x 0.97, not LaserRange). LRKeep: while fighting with a laser held, the
	// preferred laser distance is max(LaserRange, LRKeep) instead of giddy's 85 m. 0 = off.
	LRLaser float64
	LRHP    float64
	LRKeep  float64
	// RegenHide [regenhide]: FFA-only (ffaOn). Below this HP share with no target or foe (and not late), latch
	// regen-hide: coverMove from every mech seen in the last 20 s (giddy's cover uses only the last 2.5 s), stand
	// still once hidden, and skip crate farming and the loot choice (dealing damage restarts the 5 s regen
	// delay). Released at 90% HP, after 30 s, when late or when ffaOn drops; a foe or a zoneShould move takes
	// precedence while it holds. 0 = off.
	RegenHide float64
	// WatchLast [watchlast]: FFA-only (ffaOn). Idle aim (no target, nothing farmed, no engaged/hit-us mech to
	// watch, no recent damage or facesound bearing): face the nearest mech seen in the last WatchLast s within
	// 200 m with a ±0.9 rad (±52°) sweep, so it stays inside the 120° view wedge, instead of the walking sweep
	// around the heading or the 3.5 rad/s standing spin. 0 = off.
	WatchLast float64
	// HomeLOS [homelos]: FFA-only (ffaOn). homeCost adds, for each of the (up to 4, nearest us) mechs seen in the
	// last 20 s whose eye has line of sight (W.Clear) to our eye height at the candidate spot, HomeLOS if it is
	// 85-200 m from the spot and 5 x HomeLOS within 85 m (exchange rate from the calm-mech snapshots), capped at
	// 15 x HomeLOS per spot; giddy's static exposure term is unchanged. home/holdHome re-choose at once (not on the
	// 12 s timer) when a mech seen this tick gains sight of the current home. Home-spot selection only: loot, cube
	// and zone moves run before home in step and are untouched. At most 4 LOS checks per candidate. 0 = off.
	HomeLOS float64
	// ChargeEngage [chargeengage]: FFA-only (ffaOn). A visible mech charging a laser with its aim within 0.15 rad of
	// us is engaged out to this range (giddy: within 0.22 rad, to 120 m; only mechs giddy's rule leaves out are
	// added). If our laser is hot (within 0.3 s of full, in range) and it is in line of sight, the fight branch
	// fires when on target; otherwise we break its line with coverMove (boost, no new laser charge so we keep
	// full speed), unless we are finishing a target. 0 = off.
	ChargeEngage float64
	// EarlySpace [earlyspace]: ALL MODES, only before 20 s (identical in practice and FFA). Loot utility adds
	// EarlySpace x (mechs seen in the last 5 s within 85 m of the item + closeness of the nearest, 0 beyond 120 m)
	// as extra metres of distance (the sole wanted weapon pickup within 90 m is exempt); home spots add the same
	// cost and are re-chosen every 3 s instead of 12 s. 0 = off.
	EarlySpace float64
	// EarlySpaceR [earlyspacer]: earlyspace's closeness range in m (no cost from the nearest mech beyond it); 0 -> 120.
	EarlySpaceR float64
	// EarlySpaceKeep [earlyspacekeep]: with earlyspace on, before 20 s, once armed (laser or lightning held) and with
	// no foe, walk directly away from the nearest mech seen in the last 5 s while it is closer than this (m);
	// 0 -> 150 (real showdowns: top finishers keep the nearest mech ~145 m vs ~110 m). Negative = rule off.
	EarlySpaceKeep float64
	// Rescan [rescan]: ALL MODES. Idle aim only (no aim target). When calm (no damage taken for 3 s, no mech seen
	// within 50 m, no laser holder and no mech that hit us in the last 10 s within 150 m), spin a full 360 deg every 6 s
	// while gun-only and every 15 s once armed (laser or lightning); while walking, the scan alternates +/-0.9 rad
	// around the bearing to the next zone centre every 1.6 s instead of giddy's sinusoid. No jet hop. false = off.
	Rescan bool
	// LaserLOS [laserlos]: FFA-only (ffaOn). Real showdown data: team-bot lasers hit 0.93-0.97 whatever the target
	// does, but 24% of laser shots hit cover first. So dodge drops giddy's sidestep against a laser charger aimed at
	// us, and while a mech is (or may still be) holding a charge aimed at us we break its line with coverMove if the
	// cover spot is within this many metres, holding it until the charge resolves. 0 = off. (Port of lab/laserlos.)
	LaserLOS float64
	// RearAvoid [rearavoid]: PRACTICE / PRE-LATCH ONLY (acts while !ffaOn, so the post-latch FFA brain is unchanged).
	// Hits from outside the 180° front arc (centred on torso aim yaw) do x1.5 (mech.rear_mult). (1) fightMove
	// subtracts this weight per other mech we saw this tick within 170 m, with line of sight to the candidate spot,
	// that would sit outside our front arc while we aim at the target from there. (2) With no aim target, the idle
	// aim turns to the nearest mech seen this tick within 150 m that is outside our current front arc. 0 = off.
	RearAvoid float64
	// CoverReload [coverreload]: PRACTICE / PRE-LATCH ONLY (!ffaOn). While no weapon can fire (gun empty or
	// reloading with > 0.3 s left, laser absent or cooling with > 0.3 s left, lightning absent or out of
	// range/energy for the target), take cover (coverMove) from the mechs seen this tick with line of sight,
	// instead of standing in the open; not while finishing a target. false = off.
	CoverReload bool
	// FFADrops [ffadrops]: FFA-only (ffaOn). Conditional supply-drop capture through giddy's dormant drop path (the
	// UseDrops branches of chooseLoot, same values and the same "reachable before landing + 25 s" test). The value
	// is the farthest drop we go for, in m (0 = off). Only the first two drops of the match (called at 70 and
	// 140 s); the third lands near the final circle with few mechs left. See dropsOK for the calm and abort
	// guards. Real showdowns: entering a ring adds about +0.16 to P(dead within 30 s) and the capture's value
	// (full HP + energy, 3 cubes) is unproven, hence the narrow conditions. UseDrops itself is not mode-gated
	// and stays off (it would change practice play).
	FFADrops float64
	// DropAlive [dropalive]: with FFADrops, go for a drop only while at least this many mechs are alive (0 = no
	// limit). Real showdowns (Spawn chat, L9): ring entries carry about +0.2 extra death within 30 s, most with
	// few mechs left; >= 8 keeps the 70 s drop (sometimes the 140 s one) and skips the 210 s one. Unproven.
	DropAlive int
	// CubeStop [cubestop]: FFA-only (ffaOn). At or above the cube stat cap (cubes.stat_cap, 10) more cubes add no HP
	// or damage but keep growing us (+5% size each, no cap), and real showdowns show such mechs hit far more often.
	// So stop detouring for cubes and crates there (loot choice, fight-mode cube grabs, the in-fight strafe toward
	// a cube), except a cube or crate within 40 m when the next upgrade threshold (cubes.upgrade_thresholds) is
	// at most 2 cubes away. Cubes we walk over are still picked up.
	CubeStop bool
	// EarlyRetreat [earlyretreat]: FFA-only (ffaOn), only before match time EarlyRetreat s (suggested 60; 0 = off).
	// When a mech that hit us in the last 1.5 s is winning the trade (its HP > 1.15x ours and its HP fraction >
	// ours + 0.12, with us below 80% HP), or we hold only the gun and it holds a laser or lightning, withdraw to
	// cover for 5.5 s through the flee latch duelflee uses (oddsFleeUntil -> swamped -> retreat -> coverMove),
	// unless we are finishing a target. Evidence (Data/Team profiles.md): mechs that leave early fights die far
	// less before 60 s; we won 6 of 26 decisive fights in which we were opened on.
	EarlyRetreat float64
	// LCDPunish [lcdpunish]: PRACTICE-side (acts only while !ffaOn: practice and before the FFA proof). A laser
	// cannot fire again for its cooldown (weapons.laser.cooldown, 1.65 s) after a shot. For LCDPunish s after we
	// see a visible laser holder fire (its beam, or a laser sound within 0.35 rad of its bearing), engage it if our
	// HP >= its HP (in LOS, within reach), prefer it as target and close to <= 50 m. Outside that window, do not
	// open on a laser holder: a fresh one (no hits either way for 6 s) that is not aiming at us, not below 350 HP
	// and not in the end game is not engaged. 0 = off (suggested 1.5, a little under the cooldown).
	LCDPunish float64
	// SqBackOpen [sqbackopen]: SQUAD MODE ONLY (pracOn). House-bot lobby study: opening on a house bot facing more
	// than 1 rad away wins 82% vs 58% facing us; opening below 0.8x its HP wins 28%. Engage (and score -60) a seen mech
	// with LOS whose aim is > 1.0 rad off us, within our reach, when our HP >= 0.8x its HP. Do not open on a mech
	// aiming at us (giddy's 0.22 rad) while our HP < 0.8x its HP and neither side hit the other in 6 s (not late):
	// it is dropped from engagement and, with no target, we break its LOS (coverMove). false = off.
	SqBackOpen bool
	// SqLRLaser [sqlrlaser]: SQUAD MODE ONLY (pracOn). lrlaser's long-range opener in squad mode: with a ready laser
	// and HP >= its HP, open on a seen mech with LOS at EngageDist..min(SqLRLaser, MaxTarget) m whose aim is > 1.0 rad
	// off us (or, if it faces us, that is busy with another mech), at most 1 other mech within 150 m; follow through
	// 4 s after each hit. Also a regenhide-style latch below 50% HP with no foe: cover from mechs seen in 20 s,
	// stand still to 90%. 0 = off.
	SqLRLaser float64
	// LightPad [lightpad]: PRACTICE-side (only while !ffaOn). Lightning hits only within weapons.lightning.range
	// (45 m) of its 30 degree cone, so stay farther than range + LightPad m from mechs holding it: the preferred
	// fight range against a lightning holder is at least that (also when we hold lightning ourselves, and when
	// finishing), so fightMove backs off (kites) when it closes; with a laser we fight it at our laser range.
	// Candidate fight positions closer than that to any other seen lightning holder are penalised. 0 = off
	// (suggested 12).
	LightPad float64
	// SmartFire [smartfire]: PRACTICE-side (only while !ffaOn). Fire control from the exact damage rules in
	// hello.config: laser damage = damage x (min_frac + (1 - min_frac) x charge / full charge) x cube multiplier
	// x cook-off (+60% vs a mortar holder) x rear_mult (1.5) when we are in the target's rear arc. Early laser
	// release when that damage x SmartFire >= the target's HP (giddy's Lethal: squad mode only, factor 0.7, rear
	// only with Backstab); and "finishing" (commit: no retreat or flee, close to Finish range) also when a ready
	// laser at full charge, or a short gun burst (<= 1 s, in ammo, within 60 m), kills it at that safety factor.
	// The value is the safety factor (suggested 0.92: one unseen armour stack); 0 = off.
	SmartFire float64
	// ThreatFocus [threatfocus]: PRACTICE-side (only while !ffaOn). The house squad's members differ a lot in skill
	// (aim tiers), so the mech that hurts us most is not the nearest or the weakest. Keep, per attacker id, the
	// weapon damage it dealt us (damaged events) decayed with a 6 s time constant; its rate (HP/s, accumulated
	// damage / 6) times ThreatFocus is subtracted from the target score (giddy's score is in metres: distance +
	// HPw x HP - bonuses), so we turn on whoever is out-damaging us (Smith's rule: kill first what deals the most
	// damage per second of our time). 0 = off (suggested 1-2).
	ThreatFocus float64
	// CornerCover [cornercover]: PRACTICE-side (only while !ffaOn). When we retreat (LoseRule / swamped / low HP)
	// and coverMove finds no spot hidden from every mech seen in the last 2.5 s (it checks the 4 nearest, within
	// ~30 m of path), giddy stands and fights ("cornered"), where we take about twice the damage rate of a normal
	// fight. Instead, look for cover from only the CornerCover most dangerous mechs: those that see us now or hit us
	// in the last 3 s, ranked by their recent damage rate on us (threatfocus bookkeeping) and then by distance.
	// If such a spot exists, go there (mode "partcover", boost as for cover); else fight as before. 0 = off
	// (suggested 2).
	CornerCover int
	// CautiousT [cautioust]: PRACTICE-side. From match time CautiousT s, or once the next zone circle is <= 72 m,
	// squad (practice hunt) params switch off for the rest of the match and giddy's own cautious ghost play takes
	// over (engage within EngageDist or when attacked, home spots in cover, our end-game hold-out). It acts only
	// while practice params would be on, so after a proven FFA (squad params already off) it changes nothing.
	// 0 = off (suggested 265: Apex-Go's late-cautious switch; "+0.35 official" credited to Th3r0n's version).
	CautiousT float64
	// PracRim [pracrim]: PRACTICE-side (only while !ffaOn, under squad params). With no target, loot or zone move,
	// giddy's squad play stands still. While the next zone circle is >= 180 m, walk instead to the rim spot at
	// PracRim x its radius from its centre, on our side, and hold there: the house squad converges on the
	// middle, the rim keeps one side open and the zone at our back predictable. 0 = off (suggested 0.88; idea from
	// Apex-Go PracticeRim, 88% rim while r >= 180, +0.16 in its practice stack, and 4141's 0.9 rim play).
	PracRim float64
	// SquadRim family [squadrim=1 turns on all four parts at their suggested values]: PRACTICE-side, acts ONLY
	// while squad params are on (pracOn), so base/ghost play and play after the FFA latch are unchanged. Our own
	// code from the 4141 behaviour spec (vault Data/Rival code survey.md, squad phase).
	//
	// SqRim [sqrim]: idle squad play (no fight, loot or urgent zone move) holds a spot SqRim x the next circle's
	// radius from its centre while that radius is >= 200 m (0.6 x below 200 m, 0.35 x below 50 m; always >= 8 m
	// inside the circle), drifting along the rim away from the mechs seen in the last 20 s (push <= 0.5 rad).
	// It replaces giddy's idle zone walk and stand-still (and pracrim) while on. 0 = off (squadrim: 0.9).
	SqRim float64
	// SqLootIn [sqloot]: loot lying inside 0.81 x the next circle's radius from its centre is considered only
	// within SqLootIn m of us (laser and lightning pickups exempt), so loot runs do not pull us into the middle
	// where the house squad roams. 0 = off (squadrim: 50).
	SqLootIn float64
	// SqBand [sqband]: against mechs, fightMove scores our range as a band (no penalty inside it) instead of one
	// distance: gun 85-125 m (house guns miss past ~80 m), laser 110-175 m, each capped 15 m inside the weapon's
	// config range. Inactive while finishing, with lightning, or when another rule moved the range (lcdpunish,
	// lightpad, lightning/mortar holders). Off by default (squadrim: on).
	SqBand bool
	// SqRet [sqret]: squad-mode HP retreat. giddy's RetreatHP (0.38) never acts under squad params: LoseRule
	// overwrites the retreat flag every tick. SqRet adds a latch on top of LoseRule: below SqRet with a threat
	// in the last 2 s we retreat until RecoverHP (0.8), or 7 s with no threat above 55%. 0 = off (squadrim: 0.30).
	SqRet float64
	// SqRetUp [sqretup]: with SqRet on, the HP share at which the squad-mode retreat ends (instead of RecoverHP,
	// 0.8). Apex-Go's practice stance retreats at 20% and returns at 65%: back in the fight sooner, more kills.
	// 0 = use RecoverHP.
	SqRetUp float64
	// PEngage [pengage]: PRACTICE / PRE-LATCH engage radius (m). While giddy's FFA latch has not fired (b.notPractice
	// false: all of practice, including the ghost windows before PracAt and after CautiousT, and an FFA only before
	// its first death we did not cause), EngageDist = PEngage instead of 60; after the latch the base value applies,
	// so post-latch FFA play is unchanged. The practice sim found global engage=80 at about +0.2 vs x5, mostly from
	// the non-squad windows. 0 = off.
	PEngage float64
	// SqCorner [sqcorner]: with SqRet on, after more than 2 s of squad-mode retreat, if exactly one mech seen this
	// tick has line of sight to us within 45 m (a lone chaser) and our odds against it are above 0.35, stop
	// retreating for this tick and fight it (running from a lone close chaser just gives it free hits). false = off.
	SqCorner bool
	// SqSpeed [sqspeed] / SqDodge [sqdodge]: PRACTICE-side, act only while !ffaOn (practice squad play AND the
	// pre-latch window, including the FFA final before the latch; after it, giddy's ffaOn-gated SpeedUp/DodgeBoost
	// flags apply as before). House guns deal most of their damage inside 100 m, so speed and evasive boosts should
	// pay against the squad. SqSpeed: pick upgrades in SpeedUp's order (servos, jets, reactor after plating/armor).
	// SqDodge: when dodge moves us off a shot, boost if energy > SqDodge (DodgeBoost's rule). Not touched by
	// squadParams. Old-scorer board signal: S8 speedup 5.73, S7 dodgeboost=25 5.63 vs plain giddy salt mean 5.41
	// (sd 0.12). Off by default (suggested sqspeed=1, sqdodge=25).
	SqSpeed bool
	SqDodge float64
	// SqEngage [sqengage]: PRACTICE-side, squad mode only (squadParams, i.e. while pracOn). EngageDist override:
	// healthy, calm mechs inside this many metres are engaged (giddy: 60). Unlike the global engage flag it leaves
	// ghost play (before PracAt, after CautiousT) and FFA play unchanged. 0 = off (suggested 80, from the practice
	// sweep's engage=80).
	SqEngage float64
	// SqHunt [sqhunt]: SQUAD MODE ONLY (pracOn). Practice points ~ house bots we kill (rival survey K2, mechanics
	// only). Engage any mech seen within SqHunt m while our HP > 60% and our odds (HP x damage multiplier, ours over
	// its) >= 0.7; pick the nearest (target score = distance; giddy's HP term dropped, Stick and the hit-us bonus
	// kept); with no target in LOS, walk to the nearest such hidden mech; with only the gun (no laser/lightning),
	// fight it from 18-42 m (preferred range = its distance clamped to that band). 0 = off.
	SqHunt float64
	// RegenLock [regenlock]: PRACTICE-side, squad mode only (pracOn). Practice points are house bots we kill, and
	// damage that does not finish a kill comes back (regen_per_s after regen_delay s without dealing or taking
	// damage). While a mech we hit is inside that window (now - our last hit on it < mech.regen_delay):
	//   - it stays engaged, and its target score drops by 0.2 per HP we landed on it in the window (max 200), plus
	//     40, so we stay on it instead of switching to a fresher one;
	//   - if no visible target is chosen and it is out of sight or out of our target range, we walk (pathfind) to
	//     its last seen position for the rest of the window, if we can get within our weapon reach of that spot
	//     before the window ends, our HP is >= RegenLockHP of max, and no other mech is seen within 60 m.
	// Off by default. RegenLockHP [regenlockhp]: chase HP floor (0 = 0.4).
	RegenLock   bool
	RegenLockHP float64
	// KiteBoost [kiteboost]: PRACTICE-side (only while !ffaOn). The house squad walks in to its own short range, where
	// it hits us far more (sim traces: a visible house bot deals ~56 HP/s at 30-50 m, ~17 at 70-90 m, ~8 at
	// 110-140 m) while our own damage holds up with range (laser exchange ratio ~2 at 30-50 m, 7-9 at 110-180 m).
	// giddy's fightMove already backs off toward prefRange, but at walk speed, so a walking hunter keeps the gap.
	// When the target is visible and more than 15 m inside our preferred range, fightMove's step has us moving away
	// from it, our energy is above KiteBoost (and above 60 while we hold lightning, which needs energy to fire),
	// and our laser is not charging (half speed), boost (19.5 vs 11.7 m/s). Not while finishing a target.
	// 0 = off (suggested 30).
	KiteBoost float64
	// KiteW [kitew]: PRACTICE-side (only while !ffaOn), companion to KiteBoost. giddy's fightMove scores a 6 m step
	// toward prefRange at 1.1/m but a sideways (strafe) step at 9, so it strafes instead of backing off and a
	// walking hunter keeps the short range. While the mech target is visible and more than 15 m inside our preferred
	// range (not finishing it), candidate steps also score KiteW per metre they add to the distance from it, so we
	// actually back off (line of sight still scores 45). 0 = off (suggested 2.5).
	KiteW float64
	// PracRegen [pracregen]: PRACTICE-side (only while !ffaOn). RegenHide's latch for practice and the pre-latch
	// window: below this HP share with no engaged target or foe (and not late), walk into cover from every mech seen
	// in the last 20 s and stand still, with no loot run or crate/NPC shooting (dealing damage restarts the 5 s regen
	// delay; regen is 45 HP/s), until 90% HP, 30 s, late, or a mode change. An engaged mech still gets fought (the
	// fight branch runs first while foe != nil). Sim traces: squad engagements started below 70% HP end in our death
	// within 15 s 22-39% of the time, against 7% from >= 90%. 0 = off (suggested 0.6).
	PracRegen float64
}

func DefaultParams() Params {
	p := Params{RetreatHP: 0.38, RecoverHP: 0.8, GunRange: 85, LightRange: 28, LaserRange: 85, EngageDist: 60, FarmNPC: true, Explore: false,
		Ghost: true, HomeExpo: 120, LootRadius: 90, CrateUntil: 150, UseDrops: false, Dodge: true, NoMortar: true,
		Peek: true, LaserFirst: false, CrowdHide: true, Practice: true}

	p.LaserFirst = true

	// Our FFA changes (local league, see the vault's experiment note): giddy's practice params switch on at
	// 60 s instead of 20 s, retreat stays allowed until the next circle is <= 25 m, and the end-game pile-up
	// at the final centre is skipped until the zone forces the bot in.
	p.PracAt = 20
	p.FFAStrict = true
	// Build a3: C1 FFA brain + laserlos, levers QCMBZ FFA facesound=1;arm=1;scan=1.6;armfirst=1;armboost=1. Flags: ffastrictn=2,gunonlyout=1,gunonlyr=130,lrlaser=180,lrkeep=110,lrhp=1.0,lightlos=1,regenhide=0.6,homelos=60,watchlast=8,laserlos=25,sqlaser=150,cautioust=265,pracrim=0.88,sqbackopen=1,sqlrlaser=180,pengage=80,facesound=1,arm=1,scan=1.6,armfirst=1,armboost=1
	p.FFAStrictN = 2
	p.GunOnlyOut = true
	p.GunOnlyR = 130
	p.LRLaser = 180
	p.LRKeep = 110
	p.LRHP = 1.0
	p.LightLOS = true
	p.RegenHide = 0.6
	p.HomeLOS = 60
	p.WatchLast = 8
	p.LaserLOS = 25
	p.SqLaserRange = 150
	p.CautiousT = 265
	p.PracRim = 0.88
	p.SqBackOpen = true
	p.SqLRLaser = 180
	p.PEngage = 80
	p.FaceSound = true
	p.Arm = true
	p.Scan = 1.6
	p.ArmFirst = true
	p.ArmBoost = true
	p.BeamFFA = false // practice-safe (S5 = 6.19) but -0.28 in the FFA sim (round 9); kept as a flag
	p.LateR = 25
	p.HoldOut = 368
	p.HoldMin = 60
	p.LateAlive = 3

	return p
}

func NewBrain(h *Hello, w *World) *Brain {
	seed := int64(h.Seed)
	if seed == 0 {
		seed = int64(h.Slot) + 1
	}
	if Salt != 0 {
		seed ^= int64(uint64(Salt) * 0x5851F42D4C957F2D)
	}
	p := DefaultParams()
	applyFlags(&p, os.Getenv("MECH_FLAGS"))
	show := isShowdownCfg(&h.Config)
	return &Brain{showCfg: show,
		W: w, Cfg: h.Config, Slot: h.Slot, rng: rand.New(rand.NewSource(seed)), P: p,
		tracks: map[int]*Track{}, npcs: map[int]*Track{}, crates: map[int]*known{}, cubes: map[int]*known{},
		pickups: map[int]*known{}, bad: map[int]float64{}, strafe: 1, lootID: -1, lastHurt: -100, lastThreatAt: -100,
		holdStart: -1, soundAt: -100, ccFailAt: -100,
	}
}

func hyp(x, y float64) float64 { return math.Hypot(x, y) }

func angDiff(a, b float64) float64 {
	d := math.Mod(a-b, 2*math.Pi)
	if d > math.Pi {
		d -= 2 * math.Pi
	} else if d < -math.Pi {
		d += 2 * math.Pi
	}
	return d
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

func (b *Brain) Act(o *Obs) (act Action) {
	act.Tick = o.Tick
	defer func() {
		if r := recover(); r != nil {
			b.Panics++
			if b.Panics <= 5 {
				fmt.Fprintln(os.Stderr, "recover: panic", b.Panics, "at t =", o.T, ":", r)
			}
			a := b.lastAim
			act = Action{Tick: o.Tick, Aim: &a}
			if b.Panics >= 3 && !b.safeMode {
				b.safeFallback()
			}
		}
	}()
	b.step(o, &act)
	if act.Aim != nil {
		a := act.Aim
		if math.IsNaN(a[0]) || math.IsInf(a[0], 0) || math.IsNaN(a[1]) || math.IsInf(a[1], 0) {
			act.Aim = nil
		} else {
			b.lastAim = *a
		}
	}
	if math.IsNaN(act.Move[0]) || math.IsNaN(act.Move[1]) || math.IsInf(act.Move[0], 0) || math.IsInf(act.Move[1], 0) {
		act.Move = [2]float64{}
	}
	return
}

func (b *Brain) scaleOf(cubes int) float64 { return 1 + b.Cfg.Cubes.SizePerCube*float64(cubes) }

func (b *Brain) center(t *Track) Vec3 {
	h := b.Cfg.Mech.Height * b.scaleOf(t.Cubes)
	if t.Kind == "npc" {
		h = b.Cfg.Npc.Height
	}
	return Vec3{t.Pos[0], t.Pos[1], t.Pos[2] + h*0.5}
}

func (b *Brain) eye(t *Track) Vec3 {
	return Vec3{t.Pos[0], t.Pos[1], t.Pos[2] + b.Cfg.Mech.EyeHeight*b.scaleOf(t.Cubes)}
}

func (b *Brain) observe(o *Obs) {
	me := &o.Self
	now := o.T
	if me.Stats.Radius == 0 {
		me.Stats = b.Cfg.Mech
	}
	eye := Vec3{me.Pos[0], me.Pos[1], me.Pos[2] + me.Stats.EyeHeight}
	muzzle := Vec3{me.Pos[0], me.Pos[1], me.Pos[2] + me.Stats.MuzzleHeight}
	b.beamCheck(o)
	for i := range o.Drops { // ffadrops: remember call order (state only)
		known := false
		for _, id := range b.dropOrder {
			known = known || id == o.Drops[i].ID
		}
		if !known {
			b.dropOrder = append(b.dropOrder, o.Drops[i].ID)
		}
	}
	for i := range o.Mechs {
		m := &o.Mechs[i]
		tab := b.tracks
		if m.Kind == "npc" {
			tab = b.npcs
		}
		t := tab[m.ID]
		if t == nil {
			t = &Track{HitMe: -100, CalmAt: -100, HurtAt: -100, TPAt: -100, LOSAt: -100, LRAt: -100, LaserAt: -100}
			tab[m.ID] = t
		}
		if (b.P.TPJoin > 0 || b.P.LRLaser > 0) && m.Kind == "mech" && now-t.Seen <= 0.5 && m.HP < t.HP-20 {
			// tpjoin / lrlaser (busy test via tpOpp): it lost HP since our last sighting and none of our hits landed after that sighting.
			if at, ok := b.myHitAt[m.ID]; !ok || at <= t.Seen {
				t.HurtAt = now
			}
		}
		t.MechSeen = *m
		t.Seen = now
		dx, dy := m.Pos[0]-me.Pos[0], m.Pos[1]-me.Pos[1]
		t.Dist = hyp(dx, dy)
		if m.Kind == "mech" && t.Dist < 200 {
			if off := math.Abs(angDiff(m.Aim[0], math.Atan2(-dy, -dx))); off > 0.6 {
				t.CalmAt = now
			} else if off < 0.22 {
				t.CalmAt = -100
			}
		}
		c := b.center(t)
		t.LOS = b.W.Clear(muzzle, c)
		if !t.LOS && m.Kind == "mech" {
			h := b.Cfg.Mech.Height * b.scaleOf(m.Cubes)
			t.LOS = b.W.Clear(muzzle, Vec3{m.Pos[0], m.Pos[1], m.Pos[2] + h*0.85})
		}
		if t.LOS {
			t.LOSAt = now
		}
		if t.Dist > 1 {
			t.Closing = -((m.Vel[0]-me.Vel[0])*dx + (m.Vel[1]-me.Vel[1])*dy) / t.Dist
		}
	}
	for id, t := range b.tracks {
		if now-t.Seen > 20 {
			delete(b.tracks, id)
		}
	}
	for id, t := range b.npcs {
		if now-t.Seen > 120 {
			delete(b.npcs, id)
		}
	}
	upd := func(tab map[int]*known, items []Item) {
		for i := range items {
			it := &items[i]
			k := tab[it.ID]
			if k == nil {
				k = &known{ID: it.ID}
				tab[it.ID] = k
			}
			k.Kind, k.Pos, k.HP, k.Seen = it.Kind, it.Pos, it.HP, now
		}
		for id, k := range tab {
			if k.Seen == now {
				continue
			}
			d := hyp(k.Pos[0]-me.Pos[0], k.Pos[1]-me.Pos[1])
			if d < 40 && now-k.Seen > 0.35 && b.W.Clear(eye, Vec3{k.Pos[0], k.Pos[1], k.Pos[2] + 1.5}) {
				delete(tab, id)
			}
		}
	}
	upd(b.crates, o.Crates)
	upd(b.cubes, o.Cubes)
	upd(b.pickups, o.Pickups)
	for i := range o.Events {
		e := &o.Events[i]
		switch e.Type {
		case "damaged":
			if e.Weapon == "npc" && e.Bearing != nil {
				b.npcHurt, b.npcBearing = now, *e.Bearing
			}
			if e.By != nil {
				b.lastHurt = now
				if e.Bearing != nil {
					b.hurtBearing = *e.Bearing
				}
				if t := b.tracks[*e.By]; t != nil {
					t.HitMe = now
				}
				if (b.P.ThreatFocus > 0 || b.P.CornerCover > 0) && e.Weapon != "npc" && e.Weapon != "zone" {
					b.noteThreat(*e.By, e.Amount, now) // threatfocus / cornercover (state only)
				}
				if n := b.npcs[*e.By]; n != nil {
					n.HitMe = now
				}
			}
		case "kill":
			b.kills++
			delete(b.tracks, e.Target)
		}
	}

	if !b.baseSet {
		b.base, b.baseSet = b.P, true
		b.startAlive = o.Alive
	}
	ffaMin := 1
	if b.P.FFAMin > 1 {
		ffaMin = int(b.P.FFAMin)
	}
	if u := b.startAlive - o.Alive - b.kills; u > b.maxUnexpl {
		b.maxUnexpl = u
	}
	if b.startAlive-o.Alive-b.kills >= ffaMin {
		b.otherDead++
	} else {
		b.otherDead = 0
	}
	if b.otherDead >= 5 {
		b.notPractice = true
	}
	pracAt := 20.0
	if b.base.PracAt > 0 {
		pracAt = b.base.PracAt
	}
	noSquad := b.base.CfgGate && b.base.CfgNoSquad && b.showCfg
	// cautioust: late practice play hands over from the squad hunt to giddy's cautious ghost play.
	cautious := b.base.CautiousT > 0 && (now >= b.base.CautiousT || (o.Zone.NextRadius > 0 && o.Zone.NextRadius <= 72))
	if on := b.base.Practice && !noSquad && !b.notPractice && now > pracAt && !cautious; on != b.pracOn {
		b.pracOn = on
		b.P = b.base
		if on {
			b.P = squadParams(b.base)
		}
	}
	// Opening (giddy, dormant): while no death we did not cause has happened, FFA is not latched and squad
	// params are not on (so in practice only before PracAt), play laser-first with LaserSeek 240 and
	// LootRadius 150, and walk home at full speed (see the openOn check in step). OpenUntil caps it by time.
	if open := b.base.Opening && (!b.base.CfgGate || b.showCfg) && !b.notPractice && !b.pracOn && b.startAlive-o.Alive-b.kills <= 0 &&
		(b.base.OpenUntil <= 0 || now < b.base.OpenUntil); open != b.openOn {
		b.openOn = open
		b.P = b.base
		if open {
			b.P.LaserFirst, b.P.LaserSeek, b.P.LootRadius = true, 240, 150
		} else if b.pracOn {
			b.P = squadParams(b.base)
		}
	}

	// pengage: practice / pre-latch engage radius. While giddy's FFA latch has not fired (all of practice, and an
	// FFA only until its first death we did not cause), EngageDist = PEngage; after the latch it is the base value,
	// so post-latch FFA play is unchanged. sqengage keeps priority in squad mode.
	if b.base.PEngage > 0 && !(b.pracOn && b.base.SqEngage > 0) {
		if !b.notPractice {
			b.P.EngageDist = b.base.PEngage
		} else {
			b.P.EngageDist = b.base.EngageDist
		}
	}

	z := &o.Zone
	if b.holdStart != z.ShrinkStart {
		b.holdStart = z.ShrinkStart
		b.hold = circle{z.Center[0], z.Center[1], z.Radius}
	} else if now <= z.ShrinkStart {
		b.hold = circle{z.Center[0], z.Center[1], z.Radius}
	}
}

func (b *Brain) zoneAt(z *Zone, t float64) circle {
	if t <= z.ShrinkStart || z.ShrinkEnd <= z.ShrinkStart {
		return b.hold
	}
	f := clamp((t-z.ShrinkStart)/(z.ShrinkEnd-z.ShrinkStart), 0, 1)
	return circle{b.hold.X + (z.NextCenter[0]-b.hold.X)*f, b.hold.Y + (z.NextCenter[1]-b.hold.Y)*f, b.hold.R + (z.NextRadius-b.hold.R)*f}
}

func (b *Brain) curZone(z *Zone) circle {
	live := circle{z.Center[0], z.Center[1], z.Radius}
	if b.now <= z.ShrinkStart {
		return live
	}
	if math.Abs(live.R-b.hold.R) > 0.01 || math.Abs(live.X-b.hold.X) > 0.01 {
		return live
	}
	return b.zoneAt(z, b.now)
}

func (b *Brain) zoneDeadline(z *Zone, x, y, pad float64) float64 {
	if hyp(x-z.NextCenter[0], y-z.NextCenter[1]) <= z.NextRadius-pad {
		return math.Inf(1)
	}
	for k := 0; k <= 24; k++ {
		t := z.ShrinkStart + (z.ShrinkEnd-z.ShrinkStart)*float64(k)/24
		c := b.zoneAt(z, t)
		if hyp(x-c.X, y-c.Y) > c.R-pad {
			return t
		}
	}
	return z.ShrinkEnd
}

func (b *Brain) zonePenalty(z *Zone, x, y float64) float64 {
	c := b.curZone(z)
	p := 0.0
	if d := hyp(x-c.X, y-c.Y) - c.R; d > -5 {
		p += 150 + 5*(d+5)
	}
	if dn := hyp(x-z.NextCenter[0], y-z.NextCenter[1]) - z.NextRadius; dn > -5 {
		soon := clamp(1-(z.ShrinkStart-b.now)/25, 0.1, 1)
		p += soon * (25 + 0.8*(dn+5))
	}
	return p
}

func (b *Brain) zoneGoal(z *Zone, x, y float64) (float64, float64) {
	margin := clamp(0.3*z.NextRadius, 3, 30)
	if z.NextRadius <= 30 {
		return z.NextCenter[0], z.NextCenter[1]
	}
	dx, dy := x-z.NextCenter[0], y-z.NextCenter[1]
	d := hyp(dx, dy)
	if d < 1 {
		return x, y
	}
	r := math.Max(0, z.NextRadius-margin-5)
	if d <= r {
		return x, y
	}
	return z.NextCenter[0] + dx/d*r, z.NextCenter[1] + dy/d*r
}

func hasWeapon(ws []*WeaponState, kind string) (int, *WeaponState) {
	for i, w := range ws {
		if w != nil && w.Kind == kind {
			return i, w
		}
	}
	return -1, nil
}

func holds(list []string, kind string) bool {
	for _, k := range list {
		if k == kind {
			return true
		}
	}
	return false
}

var upgradeOrder = []string{
	"plating", "armor", "gun_damage", "nanites", "gun_rate", "lightning_efficiency", "lightning_range", "laser_damage",
	"servos", "laser_charge", "gun_mag", "gun_velocity", "gyros", "lightning_chain", "reactor", "capacitor",
	"mortar_reload", "mortar_splash", "mortar_velocity", "sensors", "jets", "laser_pierce",
}

var upgradeOrderLaser = []string{
	"plating", "armor", "laser_damage", "laser_charge", "nanites", "gun_damage", "gun_rate", "lightning_efficiency", "lightning_range",
	"servos", "gun_mag", "gun_velocity", "gyros", "lightning_chain", "reactor", "capacitor",
	"mortar_reload", "mortar_splash", "mortar_velocity", "sensors", "jets", "laser_pierce",
}

var upgradeOrderNano = []string{
	"plating", "armor", "nanites", "laser_damage", "laser_charge", "gun_damage", "gun_rate", "lightning_efficiency", "lightning_range",
	"servos", "gun_mag", "gun_velocity", "gyros", "lightning_chain", "reactor", "capacitor",
	"mortar_reload", "mortar_splash", "mortar_velocity", "sensors", "jets", "laser_pierce",
}

var upgradeOrderSpeed = []string{
	"plating", "armor", "servos", "jets", "reactor", "laser_damage", "laser_charge", "nanites", "gun_damage", "gun_rate", "lightning_efficiency", "lightning_range",
	"gun_mag", "gun_velocity", "gyros", "lightning_chain", "capacitor",
	"mortar_reload", "mortar_splash", "mortar_velocity", "sensors", "laser_pierce",
}

func pickUpgrade(of *Offer, laserUp, nanoUp, speedUp bool) int {
	order := upgradeOrder
	if laserUp {
		order = upgradeOrderLaser
	}
	if nanoUp {
		order = upgradeOrderNano
	}
	if speedUp {
		order = upgradeOrderSpeed
	}
	best, bestRank := 0, 1<<30
	for i, c := range of.Choices {
		rank := len(order)
		for r, id := range order {
			if id == c.ID {
				rank = r
				break
			}
		}
		if rank < bestRank {
			best, bestRank = i, rank
		}
	}
	return best
}

var weaponRank = map[string]int{"lightning": 3, "laser": 2, "mortar": 1}

func (b *Brain) step(o *Obs, act *Action) {
	me := &o.Self
	b.now = o.T
	now := o.T
	b.observe(o)
	strictN := 1
	if b.P.FFAStrictN > 1 {
		strictN = int(b.P.FFAStrictN)
	}
	ffaOn := !b.pracOn && (!b.P.FFAStrict || (b.notPractice && (strictN <= 1 || b.maxUnexpl >= strictN)) || (b.P.CfgGate && b.P.CfgFFA && b.showCfg))
	b.ffaNow = ffaOn
	if b.P.LCDPunish > 0 {
		b.noteLaserShots(o) // lcdpunish: who just fired a laser (state only)
	}
	if b.P.RegenLock {
		b.noteRegenHits(o) // regenlock: damage we landed per mech inside its regen window (state only)
	}
	st := &me.Stats
	x, y, zf := me.Pos[0], me.Pos[1], me.Pos[2]
	z := &o.Zone
	hpFrac := me.HP / math.Max(1, me.MaxHP)

	if o.Offer != nil && len(o.Offer.Choices) > 0 {
		p := pickUpgrade(o.Offer, b.P.LaserUp, b.P.NanoUp, (b.P.SpeedUp && ffaOn) || (b.P.SqSpeed && !ffaOn)) // sqspeed
		act.Pick = &p
	}

	_, gun := hasWeapon(me.Weapons, "gun")
	_, light := hasWeapon(me.Weapons, "lightning")
	_, laser := hasWeapon(me.Weapons, "laser")
	lateR := 70.0
	if b.P.LateR > 0 {
		lateR = b.P.LateR
	}
	lateAl := b.P.LateAlive
	if !ffaOn {
		lateAl = 3
	}
	late := z.NextRadius <= lateR || o.Alive <= lateAl

	var seen []*Track
	var target, hidden, shy *Track
	var sqAvoid []*Track // sqbackopen: mechs facing us that we must not open on (break their LOS)
	var chargeFoe *Track // chargeengage: nearest long-range charger engaged only by that flag
	bestScore := math.Inf(1)
	maxT, hpw := 215.0, 60.0/1400
	if b.P.MaxTarget > 0 {
		maxT = b.P.MaxTarget
	}
	if b.P.HPw > 0 {
		hpw = b.P.HPw
	}
	// hunthide: only while squad params are on (pracOn, so !Ghost), and only with 2+ calm mechs (see HuntHide).
	huntHide := false
	rlOn := b.P.RegenLock && b.pracOn // regenlock: squad mode only
	if b.P.HuntHide && b.pracOn && !b.P.Ghost {
		calm := 0
		for _, t := range b.tracks {
			if now-t.CalmAt < 5 {
				calm++
			}
		}
		huntHide = calm >= 2
	}
	for _, t := range b.tracks {
		if now-t.Seen > 2.5 {
			continue
		}
		t.Engage = false
		if t.Seen == now {
			seen = append(seen, t)
		} else {
			seen = append(seen, t)
			t.LOS = false
			continue
		}
		toMe := math.Atan2(y-t.Pos[1], x-t.Pos[0])
		aimed := math.Abs(angDiff(t.Aim[0], toMe)) < 0.22
		offAim := math.Abs(angDiff(t.Aim[0], toMe))
		switch {
		case b.P.Ghost:

			t.Engage = t.Dist < b.P.EngageDist || now-t.HitMe < 4 || (aimed && t.Charging && t.Dist < 120) ||
				(t.HP < 350 && t.Dist < 120) || o.Alive <= 2
			off := math.Abs(angDiff(t.Aim[0], toMe))
			// chargeengage: FFA-only. A visible laser charge aimed at us (0.15 rad) engages out to ChargeEngage m.
			if b.P.ChargeEngage > 0 && ffaOn && !t.Engage && t.Charging && off < 0.15 && t.Dist < b.P.ChargeEngage {
				t.Engage = true
				if chargeFoe == nil || t.Dist < chargeFoe.Dist {
					chargeFoe = t
				}
			}

			if b.P.Vulture > 0 && ffaOn && hpFrac > 0.75 && t.HP < b.P.Vulture*t.MaxHP && off > 0.6 && t.Dist < 165 {
				t.Engage = true
			}
			if b.P.Alert > 0 && t.LOS && t.Dist < b.P.Alert && (t.Closing > 3 || aimed) {
				t.Engage = true
			}
			if b.P.Shy > 0 && t.LOS && t.Dist < b.P.Shy && t.Dist >= b.P.EngageDist && (t.Closing > 3 || aimed) && !t.Engage &&
				(shy == nil || t.Dist < shy.Dist) {
				shy = t
			}

			if b.P.Ambush > 0 && hpFrac > 0.85 && off > 1.75 && t.Dist < b.P.Ambush {
				t.Engage = true
			}
			if b.P.Odds > 0 && !t.Engage && t.LOS && hpFrac > 0.5 && t.Dist < b.reach(me) && b.odds(me, t, now, true) >= b.P.Odds {
				t.Engage = true
			}
			// tpjoin: FFA-only. Join a fight it is having with another mech, from farther out than that mech.
			if b.P.TPJoin > 0 && ffaOn {
				tpHP := b.P.TPHP
				if tpHP <= 0 {
					tpHP = 0.7
				}
				if t.LOS && hpFrac >= tpHP && t.HP <= me.HP && t.Dist < b.reach(me) && off > 0.3 && now-t.HitMe > 4 {
					if opp, od := b.tpOpp(o, t); opp != nil && t.Dist > od+b.P.TPJoin {
						t.TPAt, t.OppDist = now, od
					}
				}
				if now-t.TPAt < 1.5 { // short hold bridges aim-cone flicker while they strafe
					t.Engage = true
				}
			}
			// lrlaser: FFA-only. TigerTown27-style long-range opener with a ready laser (see LRLaser).
			if b.P.LRLaser > 0 && ffaOn && !t.Engage {
				if b.lrOpen(o, me, t, laser, aimed, off, maxT) {
					t.LRAt = now
				}
				hit, ok := b.myHitAt[t.ID]
				follow := ok && now-hit < 4 && now-t.LRAt < 12 && me.HP >= b.lrHP()*t.HP &&
					t.Dist <= math.Min(b.P.LRLaser, maxT) && !b.retreat
				if now-t.LRAt < 1.5 || follow {
					t.Engage = true
				}
			}
		case b.P.AllFoes && t.Dist < maxT:
			t.Engage = true
		case t.Dist < b.P.EngageDist, now-t.HitMe < 6, late:
			t.Engage = true
		case t.Closing > 3 && t.Dist < 180:
			t.Engage = true
		case t.HP < 0.4*t.MaxHP && t.Dist < 160:
			t.Engage = true
		case aimed && t.Dist < 190:
			t.Engage = true
		}
		if b.P.SqBackOpen && b.pracOn && t.Kind == "mech" {
			// sqbackopen: open on backs, never open from behind on HP into a mech facing us.
			if offAim > 1.0 && t.LOS && t.Dist < b.reach(me) && me.HP >= 0.8*t.HP {
				t.Engage = true
			}
			hit, ok := b.myHitAt[t.ID]
			fresh := now-t.HitMe >= 6 && (!ok || now-hit >= 6)
			if fresh && aimed && me.HP < 0.8*t.HP && !late && o.Alive > 2 {
				t.Engage = false
				sqAvoid = append(sqAvoid, t)
			}
		}
		if b.P.SqLRLaser > 0 && b.pracOn && t.Kind == "mech" && !t.Engage {
			// sqlrlaser: squad-mode long-range laser opener on an unaware (or busy) mech.
			if b.sqLROpen(o, me, t, laser, offAim, maxT) {
				t.LRAt = now
			}
			hit, ok := b.myHitAt[t.ID]
			follow := ok && now-hit < 4 && now-t.LRAt < 12 && me.HP >= t.HP && t.Dist <= math.Min(b.P.SqLRLaser, maxT) && !b.retreat
			if now-t.LRAt < 1.5 || follow {
				t.Engage = true
			}
		}
		if b.sqHuntOK(me, t, hpFrac) {
			t.Engage = true // sqhunt: hunt close in squad mode
		}
		if b.P.LCDPunish > 0 && !ffaOn && t.Kind == "mech" && holds(t.Weapons, "laser") {
			// lcdpunish: punish the laser's cooldown, do not open into a ready laser (see LCDPunish).
			if b.lcdWindow(t) && me.HP >= t.HP && t.LOS && t.Dist < b.reach(me) {
				t.Engage = true
			} else if t.Engage && !b.lcdWindow(t) && !aimed && t.HP >= 350 && !late && o.Alive > 2 && now-t.HitMe >= 6 {
				if hit, ok := b.myHitAt[t.ID]; !ok || now-hit >= 6 {
					t.Engage = false
				}
			}
		}
		if huntHide && t.Engage && !(t.Dist < b.P.EngageDist || now-t.HitMe < 4 || (aimed && t.Charging && t.Dist < 120) ||
			(t.HP < 350 && t.Dist < 120) || o.Alive <= 2) {
			t.Engage = false // squad engagement that giddy's FFA ghost rule would not take
		}
		rlDmg := -1.0 // regenlock: damage we landed on it inside its live regen window (-1 = none)
		if rlOn {
			if w := b.regenLive(t.ID); w != nil {
				t.Engage, rlDmg = true, w.dmg
			}
		}
		if t.Engage && !t.LOS && t.Dist < maxT && (hidden == nil || t.Dist < hidden.Dist) {
			hidden = t
		}
		if !t.Engage || !t.LOS || t.Dist > maxT {
			continue
		}
		s := t.Dist + hpw*t.HP
		if b.P.SqHunt > 0 && b.pracOn {
			s = t.Dist // sqhunt: nearest first, no HP term
		}
		if b.P.LowBonus > 0 && t.HP <= 0.3*t.MaxHP {
			s -= b.P.LowBonus
		}
		if b.P.Stick > 0 && t.ID == b.stickID && now-b.stickAt < 2 {
			s -= b.P.Stick
		}
		if b.P.Backstab > 0 && b.showsBack(t, x, y) {
			s -= b.P.Backstab
		}
		if now-t.HitMe < 4 {
			s -= 40
		}
		if b.P.LCDPunish > 0 && !ffaOn && b.lcdWindow(t) && me.HP >= t.HP {
			s -= 60 // lcdpunish: its laser is cooling down, take it now
		}
		if b.P.ThreatFocus > 0 && !ffaOn {
			s -= b.P.ThreatFocus * b.threatRate(t.ID) // threatfocus: whoever is out-damaging us first
		}
		if t.Closing > 3 {
			s -= 15
		}
		if b.P.SqBackOpen && b.pracOn && offAim > 1.0 {
			s -= 60 // sqbackopen: its back is turned
		}
		if rlDmg >= 0 {
			s -= 40 + math.Min(200, 0.2*rlDmg) // regenlock: stay on it until its regen window runs out
		}
		if s < bestScore {
			bestScore, target = s, t
		}
	}
	if target != nil {
		b.stickID, b.stickAt = target.ID, now
	}
	foe := target
	if foe == nil && b.P.Peek {
		foe = hidden
	}
	if foe == nil && hidden != nil && b.sqHuntOK(me, hidden, hpFrac) {
		foe = hidden // sqhunt: walk to the nearest huntable mech out of sight
	}
	if foe == nil && b.P.Pursue > 0 && hidden != nil && hidden.HP < b.P.Pursue*hidden.MaxHP {
		foe = hidden
	}
	var rlChase *Track // regenlock: hurt mech out of sight (or range) to walk after inside its regen window
	if rlOn && target == nil && foe == nil {
		rlChase = b.regenChase(me, hpFrac)
	}
	danger := false
	crowd := 0
	for _, t := range seen {
		if t.Engage || now-t.HitMe < 4 {
			danger = true
		}
		if t.Seen == now && (t.Engage || (t.LOS && t.Dist < 150)) {
			crowd++
		}
	}
	if danger || now-b.lastHurt < 3 {
		b.lastThreatAt = now
	}

	finishing := target != nil && target.Dist < 120 && (target.HP < 200 || target.HP < me.HP*0.5)
	// laserlos: mechs holding (or maybe still holding, out of sight) a laser charge aimed at us; lzHot = our own laser
	// is hot on one of them with a clear line, so the fight branch shoots instead of hiding.
	var lzThreats []*Track
	lzHot := false
	if b.P.LaserLOS > 0 && ffaOn {
		lzThreats = b.laserThreats(o)
		for _, t := range lzThreats {
			if t.Seen == now && t.LOS && laser != nil && laser.Charge >= laser.Stats.Charge-0.3 && t.Dist <= laser.Stats.Range-5 {
				lzHot = true
			}
		}
	}
	if b.P.SmartFire > 0 && !ffaOn && target != nil && !finishing && b.canKill(me, target, gun, laser, x, y) {
		finishing = true // smartfire: commit when our burst kills it
	}
	_, myLaser := hasWeapon(me.Weapons, "laser")
	outgunned := target != nil && holds(target.Weapons, "laser") && myLaser == nil && target.Dist > 55 && hpFrac < 0.8
	if b.P.GunOnlyOut && ffaOn && gun != nil && light == nil && laser == nil {
		if _, mortar := hasWeapon(me.Weapons, "mortar"); mortar == nil {
			for _, t := range seen {
				if t.Seen == now && (b.P.GunOnlyR <= 0 || t.Dist < b.P.GunOnlyR) && (holds(t.Weapons, "laser") || holds(t.Weapons, "lightning")) {
					outgunned = true
					break
				}
			}
		}
	}
	swamped := b.P.CrowdHide && (crowd >= 3 || (crowd == 2 && hpFrac < 0.65) || outgunned)
	if b.P.OddsFlee > 0 && target != nil && !finishing && b.odds(me, target, now, false) < b.P.OddsFlee {
		b.oddsFleeUntil = now + 5
	}
	// duelflee: FFA-only (!pracOn); same flee latch as OddsFlee.
	if b.P.DuelFlee > 0 && ffaOn && target != nil && !finishing && b.duelOdds(me, target) < b.P.DuelFlee {
		b.oddsFleeUntil = now + 5
	}
	// earlyretreat: FFA-only, early game; same flee latch.
	if b.P.EarlyRetreat > 0 && ffaOn && now < b.P.EarlyRetreat && !finishing && b.earlyLosing(me, hpFrac, light == nil && laser == nil) {
		b.oddsFleeUntil = now + 5.5
	}
	if now < b.oddsFleeUntil && !finishing {
		swamped = true
	}
	// lightlos: FFA-only (!pracOn). foe = target, else the hidden foe Peek would step out to; using it keeps us
	// from peeking back into a laser's line once we have broken it.
	if b.P.LightLOS && ffaOn && light != nil && laser == nil && foe != nil && !finishing &&
		holds(foe.Weapons, "laser") && foe.Dist > light.Stats.Range+5 {
		swamped = true
	}
	if !b.retreat && ((hpFrac < b.P.RetreatHP && now-b.lastThreatAt < 2) || swamped) {
		b.retreat = true
		b.coverAt = -1
	}
	if b.retreat && !swamped && (hpFrac > b.P.RecoverHP || (now-b.lastThreatAt > 7 && hpFrac > 0.55)) {
		b.retreat = false
	}
	if b.P.LoseRule {
		near := 0
		for _, t := range seen {
			if t.Seen == now && t.Dist < 160 {
				near++
			}
		}
		losing := (near >= 2 && hpFrac < 0.55) || (near >= 3 && hpFrac < 0.8)
		if target != nil {
			tf := target.HP / math.Max(1, target.MaxHP)
			if hpFrac < 0.35 && tf > hpFrac+0.1 {
				losing = true
			}
			if tf < 0.25 && hpFrac > 0.2 && near <= 2 {
				losing = false
			}
		}
		if losing {
			b.loseAt = now
		}
		b.retreat = now-b.loseAt < 1.5 && b.loseAt > 0
	}
	// sqret: squad-mode HP retreat latch on top of LoseRule (see SqRet).
	if b.P.SqRet > 0 && b.pracOn {
		if !b.sqRetOn && hpFrac < b.P.SqRet && now-b.lastThreatAt < 2 {
			b.sqRetOn = true
			b.sqRetAt = now
			if !b.retreat {
				b.coverAt = -1
			}
		}
		recover := b.P.RecoverHP
		if b.P.SqRetUp > 0 {
			recover = b.P.SqRetUp // sqretup: return to the fight earlier than RecoverHP
		}
		if b.sqRetOn && (hpFrac > recover || (now-b.lastThreatAt > 7 && hpFrac > math.Min(0.55, recover))) {
			b.sqRetOn = false
		}
		if b.sqRetOn {
			b.retreat = true
			if b.P.SqCorner && now-b.sqRetAt > 2 {
				// sqcorner: turn on a lone close chaser instead of running from it.
				var lone *Track
				n := 0
				for _, t := range seen {
					if t.Kind == "mech" && t.Seen == now && t.LOS && t.Dist < 45 {
						n++
						lone = t
					}
				}
				if n == 1 && b.odds(me, lone, now, false) > 0.35 {
					b.retreat = false
				}
			}
		}
	} else {
		b.sqRetOn = false
	}
	// regenhide: FFA-only. Latch below RegenHide with no target/foe; release at 90% HP, after 30 s, when late or
	// when FFA play stops (see RegenHide).
	sqRegen := b.P.SqLRLaser > 0 && b.pracOn // sqlrlaser: regenhide-style latch in squad mode below 50% HP
	if ((b.P.RegenHide > 0 && ffaOn && hpFrac < b.P.RegenHide) || (sqRegen && hpFrac < 0.5)) && !late && !b.regenOn && foe == nil {
		b.regenOn, b.regenAt = true, now
	}
	// pracregen: the same latch while !ffaOn (practice / pre-latch), below PracRegen.
	if b.P.PracRegen > 0 && !ffaOn && !late && !b.regenOn && hpFrac < b.P.PracRegen && foe == nil {
		b.regenOn, b.regenAt = true, now
	}
	regenOK := (b.P.RegenHide > 0 && ffaOn) || sqRegen || (b.P.PracRegen > 0 && !ffaOn)
	if b.regenOn && (!regenOK || late || hpFrac >= 0.9 || now-b.regenAt > 30) {
		b.regenOn = false
	}

	exposed := false
	for _, t := range seen {
		if t.Seen == now && t.LOS && t.Dist < 230 {
			exposed = true
		}
	}
	for _, t := range seen {
		if t.Seen == now && t.Dist < 45 {
			exposed = false
		}
	}

	var mx, my float64
	boost := false
	mode := "idle"
	var farm *Track
	var farmPos *Vec3

	zgx, zgy := b.zoneGoal(z, x, y)
	zdist := hyp(zgx-x, zgy-y)
	tNeed := zdist*1.45/st.WalkSpeed + 4
	deadline := b.zoneDeadline(z, x, y, 8)
	zoneMust := !me.InZone || now+tNeed+3 >= deadline
	zoneShould := now+tNeed+14 >= deadline

	escJump := false
	switch {
	case b.escape(me, &mx, &my, &boost, &escJump):
		mode = "escape"
	case now < b.unstuckUntil:
		mx, my = b.unstuckDir[0], b.unstuckDir[1]
		mode = "unstuck"
	case b.dodgeShell(o, &mx, &my):
		boost = me.Energy > 15
		mode = "dodge"
	case zoneMust && zdist > 2:
		mx, my = b.travel(me, zgx, zgy)
		boost = (!me.InZone || now+tNeed >= deadline) && me.Energy > 5
		mode = "zone"
	case b.retreat && !finishing && !late:
		if b.coverMove(o, seen, &mx, &my) {
			boost = me.Energy > 10 && (mx != 0 || my != 0)
			mode = "cover"
		} else if b.P.CornerCover > 0 && !ffaOn && b.cornerCover(o, seen, &mx, &my) {
			// cornercover (practice / pre-latch only): no cover close by, so search farther, then hide from the
			// worst attackers only.
			boost = me.Energy > 10 && (mx != 0 || my != 0)
			mode = "partcover"
		} else if target != nil {
			mx, my = b.fightMove(o, target, seen, 1)
			mode = "cornered"
		} else {
			mx, my = b.travel(me, zgx, zgy)
			if b.npcHurt > 0 && now-b.npcHurt < 3 && mx == 0 && my == 0 {

				side := 1.0
				if math.Mod(now, 3) > 1.5 {
					side = -1
				}
				mx, my = -math.Sin(b.npcBearing)*side, math.Cos(b.npcBearing)*side
			}
			mode = "flee"
		}
	case target == nil && now-b.lastHurt < 2.5 && !late && b.P.Ghost && b.hideFromBearing(o, seen, &mx, &my):
		boost = me.Energy > 30 && (mx != 0 || my != 0)
		mode = "cover"
	case b.regenOn && foe == nil && !(zoneShould && zdist > 2) && b.coverMove(o, b.recentMechs(x, y, 20), &mx, &my):
		// regenhide: walk (no boost) into cover from every mech seen in the last 20 s; coverMove stands still
		// once hidden. With no such mech within 230 m (or no cover) we fall through to home, without loot.
		mode = "regen"
	case now < b.P.EarlyHide && exposed && !finishing && b.coverMove(o, seen, &mx, &my):
		boost = me.Energy > 20 && (mx != 0 || my != 0)
		mode = "earlyhide"
	case foe == nil && shy != nil && !late && b.coverMove(o, seen, &mx, &my):
		boost = me.Energy > 40 && (mx != 0 || my != 0)
		mode = "shy"
	case b.P.Evade && foe != nil && !late && !finishing && foe.Dist > b.P.EngageDist*1.2 && foe.HP > 0.45*foe.MaxHP &&
		b.coverMove(o, seen, &mx, &my):
		boost = me.Energy > 30 && (mx != 0 || my != 0)
		mode = "evade"
	case len(lzThreats) > 0 && !lzHot && !finishing && b.laserCover(o, lzThreats, &mx, &my):
		// laserlos: break (and hold) the line of every laser charge aimed at us when cover is close enough.
		boost = me.Energy > 20 && (mx != 0 || my != 0)
		mode = "lasercover"
	case b.P.CoverReload && !ffaOn && !finishing && !b.canFire(me, gun, laser, light, target) &&
		b.coverMove(o, losNow(seen, now), &mx, &my):
		// coverreload (practice / pre-latch only): nothing can fire, so hide from the mechs that see us.
		boost = me.Energy > 20 && (mx != 0 || my != 0)
		mode = "reloadcover"
	case chargeFoe != nil && !finishing && !(chargeFoe.LOS && laser != nil && laser.Charge >= laser.Stats.Charge-0.3 &&
		chargeFoe.Dist <= laser.Stats.Range-5) && b.coverMove(o, seen, &mx, &my):
		// chargeengage: no hot laser on the long-range charger, so break its line (coverMove) instead of fighting.
		boost = me.Energy > 20 && (mx != 0 || my != 0)
		mode = "chargecover"
	case b.P.EarlySpace > 0 && now < b.earlySpaceT() && foe == nil && (laser != nil || light != nil) && b.earlyKeep(me, &mx, &my):
		mode = "earlyspace" // walk away from the nearest recently seen mech until it is EarlySpaceKeep m away
	case foe != nil && b.armRunUnderFire(o, me, foe, hpFrac, danger, &mx, &my):
		mode = "arm:underfire" // armfirst: keep running to a weapon while the shooter is far
	case len(sqAvoid) > 0 && target == nil && !finishing && b.coverMove(o, sqAvoid, &mx, &my):
		boost = me.Energy > 20 && (mx != 0 || my != 0)
		mode = "sqavoid" // sqbackopen: break the LOS of a facing mech we are too weak to open on
	case rlChase != nil:
		// regenlock: walk to where the hurt mech was last seen while its regen window is still open.
		mx, my = b.travel(me, rlChase.Pos[0], rlChase.Pos[1])
		mode = "regenchase"
	case foe != nil:

		ready := gun != nil && gun.Ammo > 0 && gun.Reload <= 0.25 && foe.Dist <= gun.Stats.Range-10
		hot := laser != nil && laser.Charge >= laser.Stats.Charge-0.3 && foe.Dist <= laser.Stats.Range-5
		if hot || (light != nil && foe.Dist <= light.Stats.Range+6 && me.Energy >= light.Stats.Energy*3) {
			ready = true
		}
		if !b.P.Peek && (light != nil || laser != nil) {
			ready = true
		}
		wantLos := -1.0
		if ready {
			wantLos = 1
		}
		if b.P.PeekCD != 0 && ffaOn && target == nil {
			// peekcd: 1 = no peek while our laser cools down; 2 = peek during the hidden foe's laser cooldown.
			if b.P.PeekCD&1 != 0 && laser != nil && laser.Cooldown > 0.05 {
				wantLos = -1
			}
			if b.P.PeekCD&2 != 0 && holds(foe.Weapons, "laser") {
				fb := math.Atan2(foe.Pos[1]-y, foe.Pos[0]-x)
				for _, s := range o.Sounds {
					if s.Kind == "laser" && s.Dist != "far" && math.Abs(angDiff(s.Bearing, fb)) < 0.35 {
						b.cdID, b.cdUntil = foe.ID, now-s.Age+1.6
					}
				}
				if b.cdID == foe.ID && now < b.cdUntil {
					wantLos = 1
				}
			}
		}
		if b.P.Peek && !hot {
			for _, t := range seen {
				if t.Seen == now && t.Charging && math.Abs(angDiff(t.Aim[0], math.Atan2(y-t.Pos[1], x-t.Pos[0]))) < 0.3 {
					wantLos = -1.6
				}
			}
		}
		finHP := 0.38
		if b.P.FinishHP > 0 {
			finHP = b.P.FinishHP
		}
		if keep := b.P; b.P.Finish > 0 && foe.HP < finHP*foe.MaxHP && hpFrac > 0.42 && crowd <= 2 {
			b.P.GunRange, b.P.LaserRange, b.P.LightRange = b.P.Finish, b.P.Finish, b.P.Finish
			b.finMove = true // sqband: no range band while closing in to finish
			mx, my = b.fightMove(o, foe, seen, wantLos)
			b.finMove = false
			b.P = keep
			if b.P.FinishRush && foe.Dist > 2*b.P.Finish && me.Energy > 30 {
				boost = true
			}
		} else {
			if b.P.LRLaser > 0 && b.P.LRKeep > 0 && ffaOn && laser != nil {
				b.P.LaserRange = math.Max(b.P.LaserRange, b.P.LRKeep) // lrkeep: fight from long laser range
			}
			b.kiteOn = b.P.KiteW > 0 && !ffaOn && !finishing && foe.Kind == "mech" && foe.Seen == now && foe.Dist < b.prefRange(me, foe)-15
			mx, my = b.fightMove(o, foe, seen, wantLos)
			b.kiteOn = false
			b.P = keep
			if b.P.KiteBoost > 0 && !ffaOn && !finishing && b.kiteAway(me, foe, mx, my, light != nil, laser) {
				boost = true // kiteboost (practice / pre-latch only): back off to our range at boost speed
			}
		}
		if light != nil && foe.Dist > 50 && me.Energy > 55 && wantLos > 0 {
			boost = true
		}
		mode = "fight"
		if b.P.FightBoost > 0 && ffaOn && me.Energy > b.P.FightBoost && (laser == nil || laser.Charge <= 0) && (light == nil || me.Energy > 60) {
			boost = true
		}
		if b.P.FightLoot > 0 {
			r := b.P.FightLoot
			if hpFrac <= 0.5 {
				r *= 0.57
			}
			var bc *Vec3
			for _, c := range b.cubes {
				if d := hyp(c.Pos[0]-x, c.Pos[1]-y); d < r && !b.cubeSkip(me, c.Pos) {
					r = d
					cp := c.Pos
					bc = &cp
				}
			}
			if bc != nil {
				mx, my = b.travel(me, bc[0], bc[1])
				mode = "fight:cube"
			}
		}
	case zoneShould && zdist > 2:
		mx, my = b.travel(me, zgx, zgy)
		mode = "zone"
	default:
		var lc *lootChoice
		if !b.regenOn { // regenhide: no loot run (crate/NPC hits restart the regen delay) while regenerating
			lc = b.chooseLoot(o, hpFrac, danger)
		}
		if lc != nil {
			mode = "loot:" + lc.kind
			switch lc.kind {
			case "npc":
				t := b.npcs[lc.id]
				if t != nil && now-t.Seen < 1 && t.LOS && t.Dist < 75 {
					farm = t
					mx, my = b.fightMove(o, t, seen, 1)
				} else {
					mx, my = b.travel(me, lc.pos[0], lc.pos[1])
				}
			case "crate":
				d := hyp(lc.pos[0]-x, lc.pos[1]-y)
				cp := Vec3{lc.pos[0], lc.pos[1], lc.pos[2] + b.Cfg.Crates.Height*0.5}
				if d < 50 && b.W.Clear(Vec3{x, y, zf + st.MuzzleHeight}, cp) {
					farmPos = &cp
					if d < 12 {
						mx, my = (x-lc.pos[0])/math.Max(d, 0.1), (y-lc.pos[1])/math.Max(d, 0.1)
					}
				} else {
					mx, my = b.travel(me, lc.pos[0], lc.pos[1])
				}
			default:
				mx, my = b.travel(me, lc.pos[0], lc.pos[1])
				if lc.kind == "pickup" {
					b.abAt, b.abPos = now+1, [2]float64{lc.pos[0], lc.pos[1]} // armboost (stored +1 so 0 = unset)
				}
				if lc.kind == "drop" && hyp(lc.pos[0]-x, lc.pos[1]-y) < 2.5 {
					mx, my = 0, 0
				}
			}
		} else if b.P.SqRim > 0 && b.pracOn && z.NextRadius > 0 {
			// sqrim: idle squad play holds (and drifts along) the rim of the next circle; it replaces the idle
			// zone walk, since the rim spot is always inside the next circle.
			if rx, ry := b.squadRimSpot(z, x, y); hyp(rx-x, ry-y) > 6 {
				mx, my = b.travel(me, rx, ry)
				mode = "sqrim"
			}
		} else if zdist > 3 && (!b.P.Ghost || (z.NextRadius <= 30 && !b.holdingOut(z, o.Alive))) {
			mx, my = b.travel(me, zgx, zgy)
			mode = "zone-walk"
		} else if b.P.PracRim > 0 && !ffaOn && !b.P.Ghost && z.NextRadius >= 180 {
			// pracrim: idle squad play holds the rim of the next circle instead of standing still.
			if rx, ry := b.rimSpot(z, x, y); hyp(rx-x, ry-y) > 6 {
				mx, my = b.travel(me, rx, ry)
				mode = "rim"
			}
		} else if b.P.Ghost {

			if hx, hy, ok := b.home(o); ok && hyp(hx-x, hy-y) > 3 {
				mx, my = b.travel(me, hx, hy)
				if !danger && now-b.lastHurt > 6 && !b.openOn {
					q := math.Min(1, 4.6/st.WalkSpeed)
					mx, my = mx*q, my*q
				}
				mode = "home"
			}
		} else if b.P.Explore && !late && me.Cubes < b.cubeCap() && hpFrac > 0.7 && !danger {
			mx, my = b.exploreMove(o)
			mode = "explore"
		}
	}

	if escJump {
		act.Jump = true
	}
	if b.P.ArmBoost && b.abAt == now+1 && now < 30 && b.armActive(me) && me.Energy > 30 && (mx != 0 || my != 0) &&
		hyp(b.abPos[0]-x, b.abPos[1]-y) > 25 {
		boost = true // armboost: run to the weapon
	}
	if b.P.Dodge && mode != "unstuck" && mode != "escape" {
		if dx, dy, lz := b.dodge(o); dx != 0 || dy != 0 {
			mx, my = mx*0.4+dx*0.9, my*0.4+dy*0.9
			if l := hyp(mx, my); l > 1 {
				mx, my = mx/l, my/l
			}
			if lz && me.Energy > 25 {
				boost = true
			}
			if b.P.DodgeBoost > 0 && ffaOn && me.Energy > b.P.DodgeBoost {
				boost = true
			}
			if b.P.SqDodge > 0 && !ffaOn && me.Energy > b.P.SqDodge {
				boost = true // sqdodge: DodgeBoost's rule while !ffaOn
			}
		}
	}
	if b.P.ThreatBoost > 0 && ffaOn && me.Energy > b.P.ThreatBoost && (foe != nil || now-b.lastHurt < 2.5) && (light == nil || me.Energy > 60) {
		boost = true
	}

	if mode == "escape" {
		b.stuckSince = -1
	} else if (mx != 0 || my != 0) && hyp(me.Vel[0], me.Vel[1]) < 1.2 && me.Grounded {
		if b.stuckSince < 0 {
			b.stuckSince = now
		} else if now-b.stuckSince > 0.7 {
			a := b.rng.Float64() * 2 * math.Pi
			b.unstuckDir = [2]float64{math.Cos(a), math.Sin(a)}
			b.unstuckUntil = now + 0.5
			b.stuckSince = -1
			b.path = nil
			act.Jump = true
		}
	} else {
		b.stuckSince = -1
	}

	muzzle := Vec3{x + me.Vel[0]*lat, y + me.Vel[1]*lat, zf + st.MuzzleHeight}
	aim := [2]float64{me.Aim[0], 0}
	haveAim := false
	var tp, tv Vec3
	var tr, th float64
	shootable := false
	isMech := false
	defend := false
	if target == nil && b.npcHurt > 0 && now-b.npcHurt < 3 {

		var worst *Track
		bestOff := 0.7
		for _, n := range b.npcs {
			if now-n.Seen > 0.5 || !n.LOS || n.Dist > 150 {
				continue
			}
			off := math.Abs(angDiff(math.Atan2(n.Pos[1]-y, n.Pos[0]-x), b.npcBearing))
			if now-n.HitMe < 3 {
				off = 0
			}
			if off < bestOff {
				bestOff, worst = off, n
			}
		}
		if worst != nil {
			farm, farmPos, defend = worst, nil, true
		}
	}
	if b.P.Econ && target == nil && farm == nil && farmPos == nil && gun != nil && !b.retreat && !b.regenOn { // regenhide: no crates

		reach := 110.0
		for _, t := range b.tracks {
			if now-t.Seen < 4 {
				reach = 50
			}
		}
		for _, c := range b.crates {
			if until, ok := b.bad[c.ID]; ok && until > now {
				continue
			}
			d := hyp(c.Pos[0]-x, c.Pos[1]-y)
			cp := Vec3{c.Pos[0], c.Pos[1], c.Pos[2] + b.Cfg.Crates.Height*0.5}
			if d < reach && d > 6 && b.W.Clear(muzzle, cp) {
				reach = d
				farmPos = &cp
			}
		}
	}
	switch {
	case target != nil:
		sc := b.scaleOf(target.Cubes)
		tr, th = b.Cfg.Mech.Radius*sc, b.Cfg.Mech.Height*sc
		tp, tv = target.Pos, target.Vel
		tp[2] += th * 0.5
		if !b.W.Clear(muzzle, tp) {
			tp[2] = target.Pos[2] + th*0.85
		}
		shootable, isMech = true, true
	case farm != nil:
		tr, th = b.Cfg.Npc.Radius, b.Cfg.Npc.Height
		tp, tv = farm.Pos, farm.Vel
		tp[2] += th * 0.5
		shootable = true
	case farmPos != nil:
		tr, th = b.Cfg.Crates.Radius, b.Cfg.Crates.Height
		tp = *farmPos
		shootable = true
	}
	if !isMech && target == nil && (b.retreat || hpFrac < 0.5) && !b.P.Econ && !defend {
		shootable = false
	}

	if shootable {
		dist := hyp(tp[0]-x, tp[1]-y)
		grounded := math.Abs(tv[2]) < 0.5

		var gy, gp float64
		gunOK := false
		if gun != nil {
			gy, gp = b.lead(muzzle, tp, tv, gun.Stats.Speed, grounded)
			gunOK = dist <= gun.Stats.Range*0.96
		}

		dp := Vec3{tp[0] + tv[0]*(lat+0.05), tp[1] + tv[1]*(lat+0.05), tp[2]}
		hy := math.Atan2(dp[1]-muzzle[1], dp[0]-muzzle[0])
		hp := math.Atan2(dp[2]-muzzle[2], hyp(dp[0]-muzzle[0], dp[1]-muzzle[1]))
		aim = [2]float64{gy, gp}
		if gun == nil {
			aim = [2]float64{hy, hp}
		}
		haveAim = true
		tolY := math.Atan2(tr*0.75, math.Max(dist, 1))
		tolP := math.Atan2(th*0.4, math.Max(dist, 1))
		slew := func(wantY, wantP float64) bool {
			ey := math.Max(0, math.Abs(angDiff(wantY, me.Aim[0]))-st.TurnRate*0.05)
			ep := math.Max(0, math.Abs(wantP-me.Aim[1])-st.PitchRate*0.05)
			return ey < tolY && ep < tolP
		}
		for i, w := range me.Weapons {
			if w == nil || i > 1 {
				continue
			}
			switch w.Kind {
			case "gun":
				if gunOK && w.Ammo > 0 && w.Reload <= 0 && slew(gy, gp) {
					act.Fire[i] = true
				}
				if isMech && target != nil && dist > dflt(b.P.ArmFire, 45) && now-target.HitMe >= 4 && b.armActive(me) {
					act.Fire[i] = false // arm: while gun-only, do not open fire beyond ArmFire
				}
			case "lightning":
				cone := w.Stats.ConeDeg * math.Pi / 180
				if dist < w.Stats.Range-1.5 && math.Abs(angDiff(hy, me.Aim[0])) < cone*0.5-0.05 && me.Energy >= w.Stats.Energy {
					act.Fire[i] = true
				}
			case "laser":
				if !isMech || dist > w.Stats.Range*0.97 {
					break
				}
				full := w.Stats.Charge
				smart := b.P.SmartFire > 0 && !b.ffaNow
				if (b.P.Lethal || smart) && w.Charge >= w.Stats.MinCharge && w.Charge < full-0.051 {
					var dmg, lk float64
					if smart {
						dmg, lk = b.laserDmg(me, w, target, w.Charge, x, y), b.P.SmartFire // smartfire: exact damage
					} else {
						dmg = w.Stats.Damage * (w.Stats.MinFrac + (1-w.Stats.MinFrac)*w.Charge/full) *
							(1 + b.Cfg.Cubes.DamagePerCube*float64(min(me.Cubes, b.Cfg.Cubes.StatCap)))
						if holds(target.Weapons, "mortar") {
							dmg *= 1 + w.Stats.Cookoff
						}
						lk = 0.8
						if b.P.LethalK > 0 {
							lk = b.P.LethalK
						}
						if b.P.Backstab > 0 && b.showsBack(target, x, y) {
							dmg *= b.rearMult()
						}
					}
					if dmg*lk >= target.HP && math.Abs(angDiff(hy, me.Aim[0])) < tolY*0.9 && math.Abs(hp-me.Aim[1]) < tolP {
						aim = [2]float64{hy, hp}
						act.Fire[i] = false
						break
					}
				}
				if w.Charge < full-0.051 {
					act.Fire[i] = true
					if w.Charge > full*0.45 {
						aim = [2]float64{hy, hp}
					}
				} else {
					aim = [2]float64{hy, hp}
					ey := math.Abs(angDiff(hy, me.Aim[0]))
					ep := math.Abs(hp - me.Aim[1])
					if ey < tolY*0.9 && ep < tolP {
						act.Fire[i] = false
					} else {
						act.Fire[i] = true
					}
				}
			case "mortar":
				if !isMech || w.Cooldown > 0.25 || dist < 16 || dist > 200 {
					break
				}
				tube := Vec3{muzzle[0], muzzle[1], zf + st.Height + 0.5}
				foot := Vec3{tp[0], tp[1], tp[2] - th*0.5}
				if my2, mp, ok := b.mortar(tube, foot, tv, w.Stats.Speed, w.Stats.Gravity, st); ok {

					if gun == nil || gun.Reload > 0 || gun.Ammo == 0 || !gunOK {
						aim = [2]float64{my2, mp}
					}
					ey := math.Max(0, math.Abs(angDiff(my2, me.Aim[0]))-st.TurnRate*0.05)
					ep := math.Max(0, math.Abs(mp-me.Aim[1])-st.PitchRate*0.05)
					if ey < 0.04 && ep < 0.03 {
						act.Fire[i] = true
					}
				}
			}
		}
		if isMech && b.P.BackOpen > 0 && ffaOn {
			if hold, sees := b.backHold(me, target, hpFrac, laser != nil && laser.Charge > 0); hold {
				// backopen: hold the opener for its back (see backHold); aim and tracking are unchanged.
				for i, w := range me.Weapons {
					switch {
					case w == nil || i > 1:
					case w.Kind == "gun":
						act.Fire[i] = false
					case w.Kind == "laser" && w.Charge >= w.Stats.Charge-0.051:
						act.Fire[i] = true // keep the full charge (it fires on its own only hold_s after full)
					case w.Kind == "laser" && w.Charge <= 0 && sees:
						act.Fire[i] = false // never show it a charge
					}
				}
			}
		}
	} else {

	}

	if target == nil && foe != nil {

		for i, w := range me.Weapons {
			if w != nil && i < 2 && w.Kind == "laser" && w.Cooldown <= 0.05 && foe.Dist < w.Stats.Range && w.Charge < w.Stats.Charge+w.Stats.HoldS-0.35 {
				if b.P.PreCharge > 0 && ffaOn && (now-foe.LOSAt > b.P.PreCharge || foe.Dist > 80) {
					continue // precharge: no charge for a foe that left LOS long ago or is far; a held one is let go
				}
				act.Fire[i] = true
			}
		}
	}
	if mode == "chargecover" || mode == "lasercover" {
		for i, w := range me.Weapons {
			if w != nil && i < 2 && w.Kind == "laser" && w.Charge <= 0 {
				act.Fire[i] = false // chargeengage: no new charge (half speed) while breaking its line
			}
		}
	}

	if b.P.FaceSound {
		b.hearShots(o)
	}
	if !haveAim {

		var watch *Track
		for _, t := range seen {
			if t.Engage || now-t.HitMe < 6 {
				if watch == nil || t.Dist < watch.Dist {
					watch = t
				}
			}
		}
		var last *Track
		if b.P.WatchLast > 0 && ffaOn && target == nil && farm == nil && farmPos == nil {
			last = b.lastSeen(x, y) // watchlast
		}
		var rearT *Track // rearavoid (practice / pre-latch only): nearest seen mech behind our front arc
		if b.P.RearAvoid > 0 && !ffaOn {
			for _, t := range seen {
				if t.Seen == now && t.Dist < 150 && math.Abs(angDiff(me.Aim[0], math.Atan2(t.Pos[1]-y, t.Pos[0]-x))) > math.Pi/2 &&
					(rearT == nil || t.Dist < rearT.Dist) {
					rearT = t
				}
			}
		}
		switch {
		case watch != nil:
			aim[0] = math.Atan2(watch.Pos[1]-y, watch.Pos[0]-x)
		case rlChase != nil:
			aim[0] = math.Atan2(rlChase.Pos[1]-y, rlChase.Pos[0]-x) // regenlock: face where it was
		case rearT != nil:
			aim[0] = math.Atan2(rearT.Pos[1]-y, rearT.Pos[0]-x)
		case b.npcHurt > 0 && now-b.npcHurt < 2.5:
			aim[0] = b.npcBearing
		case now-b.lastHurt < 2.5:
			aim[0] = b.hurtBearing
		case b.P.FaceSound && ffaOn && target == nil && farm == nil && farmPos == nil && now-b.soundAt < 1.5:
			// facesound: FFA-only (!pracOn); below watch/npc/hurt bearings, above the walking scan.
			aim[0] = b.soundBearing
		case b.P.Scan > 0 && now < b.P.Scan:
			aim[0] = me.Aim[0] + 1.2 // scan: spin to learn the pickups around the spawn
		case b.P.Rescan && b.rescanSpin(o, seen, light != nil || laser != nil):
			aim[0] = me.Aim[0] + 1.2 // rescan: keep turning at full slew for one full turn
		case last != nil:
			// watchlast: FFA-only; keep the last-seen mech in the 120° wedge with a ±52° sweep (above the scans).
			aim[0] = math.Atan2(last.Pos[1]-y, last.Pos[0]-x) + 0.9*math.Sin(now*1.3)
		case b.P.Rescan && (mx != 0 || my != 0):
			// rescan walk-scan: +/-0.9 rad around the bearing to the next zone centre, switching every 1.6 s.
			side := 0.9
			if int(now/1.6)%2 == 1 {
				side = -0.9
			}
			aim[0] = math.Atan2(o.Zone.NextCenter[1]-y, o.Zone.NextCenter[0]-x) + side
		case mx != 0 || my != 0:
			aim[0] = math.Atan2(my, mx) + 1.15*math.Sin(now*1.3)
		default:
			aim[0] = me.Aim[0] + 0.35
		}
		aim[1] = 0
	}
	aim[1] = clamp(aim[1], st.MinPitch, st.MaxPitch)
	act.Aim = &aim

	if gun != nil && gun.Reload <= 0 && gun.Ammo < gun.Stats.Mag {
		if target == nil && farm == nil && farmPos == nil && (gun.Ammo <= gun.Stats.Mag*0.6 || now-b.lastThreatAt > 3) {
			act.Reload = true
		}
	}

	slot1 := ""
	if len(me.Weapons) > 1 && me.Weapons[1] != nil {
		slot1 = me.Weapons[1].Kind
	}
	for i := range o.Pickups {
		p := &o.Pickups[i]
		if hyp(p.Pos[0]-x, p.Pos[1]-y) > b.Cfg.Loot.PickupRadius-0.3 || math.Abs(p.Pos[2]-zf) > 3.6 {
			continue
		}
		if !b.wantPickup(me, p.Kind) {
			continue
		}
		id := p.ID
		act.Take = &id
		if slot1 != "" && len(me.Weapons) > 0 && me.Weapons[0] != nil {
			if slot1 == "laser" && me.Weapons[1].Charge > 0 {
				act.Take = nil
				break
			}
			one := 1
			act.Drop = &one
		}
		break
	}

	act.Move = [2]float64{mx, my}
	act.Boost = boost && (mx != 0 || my != 0)
	b.Mode = mode
}

func (b *Brain) escape(me *Self, mx, my *float64, boost, jump *bool) bool {
	x, y, zf := me.Pos[0], me.Pos[1], me.Pos[2]
	w := b.W
	if me.Grounded {
		if zf-w.GroundAt(x, y) > 3 && w.Obst[w.vidx(x, y)] {
			b.escOK, b.escFlying = false, false
			return false
		}
		if !w.OffMain(x, y) {
			b.escOK, b.escFlying = false, false
			return false
		}
		b.escFlying = false
		if !b.escOK || b.now-b.escAt > 6 {
			b.escLaunch, b.escLand, b.escOK = w.EscapePlan(x, y)
			b.escAt = b.now
			if !b.escOK {
				return false
			}
		}
		if hyp(b.escLaunch[0]-x, b.escLaunch[1]-y) > 2.5 {
			*mx, *my = b.travel(me, b.escLaunch[0], b.escLaunch[1])
			return true
		}
		if me.Energy < math.Min(70, me.MaxEnergy*0.7) {
			*mx, *my = 0, 0
			return true
		}
		b.escFlying = true
		*jump = true
	} else if !b.escFlying {
		return false
	}
	dx, dy := b.escLand[0]-x, b.escLand[1]-y
	d := hyp(dx, dy)
	if d < 1.5 {
		return true
	}
	*mx, *my = dx/d, dy/d
	need := -1e9
	for t := 0.0; t <= math.Min(d, 30); t += 1 {
		need = math.Max(need, w.TopAt(x+*mx*t, y+*my*t))
	}
	*boost = zf < need+2.5 && me.Energy > 1
	return true
}

func (b *Brain) wantPickup(me *Self, kind string) bool {
	if kind == "gun" {
		return len(me.Weapons) > 0 && me.Weapons[0] == nil
	}
	if i, _ := hasWeapon(me.Weapons, kind); i >= 0 {
		return false
	}
	if kind == "mortar" && b.P.NoMortar {
		return false
	}
	rank := func(k string) int {
		if (b.P.LaserFirst || (b.P.KeepLaser && b.ffaNow)) && k == "laser" {
			return 4
		}
		return weaponRank[k]
	}
	cur := 0
	if len(me.Weapons) > 1 && me.Weapons[1] != nil {
		cur = rank(me.Weapons[1].Kind)
	}
	return rank(kind) > cur
}

func (b *Brain) lead(muzzle, tp, tv Vec3, speed float64, grounded bool) (float64, float64) {
	if speed <= 0 {
		speed = 252
	}
	t := 0.0
	var px, py, pz float64
	for k := 0; k < 4; k++ {
		tt := lat + t
		px, py, pz = tp[0]+tv[0]*tt, tp[1]+tv[1]*tt, tp[2]
		if !grounded {
			pz += tv[2] * tt * 0.5
		}
		d := math.Sqrt((px-muzzle[0])*(px-muzzle[0]) + (py-muzzle[1])*(py-muzzle[1]) + (pz-muzzle[2])*(pz-muzzle[2]))
		t = math.Max(0, d-3) / speed
	}
	return math.Atan2(py-muzzle[1], px-muzzle[0]), math.Atan2(pz-muzzle[2], hyp(px-muzzle[0], py-muzzle[1]))
}

func (b *Brain) mortar(tube, foot, tv Vec3, v, g float64, st *MechStats) (yaw, pitch float64, ok bool) {
	if v <= 0 || g <= 0 {
		return 0, 0, false
	}
	t := 0.0
	for k := 0; k < 3; k++ {
		px, py := foot[0]+tv[0]*(lat+t), foot[1]+tv[1]*(lat+t)
		dx := hyp(px-tube[0], py-tube[1])
		dz := foot[2] - tube[2]
		disc := v*v*v*v - g*(g*dx*dx+2*dz*v*v)
		if disc < 0 || dx < 1 {
			return 0, 0, false
		}
		pitch = math.Atan((v*v - math.Sqrt(disc)) / (g * dx))
		yaw = math.Atan2(py-tube[1], px-tube[0])
		t = dx / (v * math.Cos(pitch))
	}
	if pitch < st.MinPitch || pitch > st.MaxPitch {
		return 0, 0, false
	}

	c, s := math.Cos(pitch), math.Sin(pitch)
	for tt := 0.1; tt < t*0.9; tt += 0.1 {
		d := v * c * tt
		px, py := tube[0]+math.Cos(yaw)*d, tube[1]+math.Sin(yaw)*d
		pz := tube[2] + v*s*tt - 0.5*g*tt*tt
		if b.W.Solid(px, py, pz) {
			return 0, 0, false
		}
	}
	return yaw, pitch, true
}

func (b *Brain) dodgeShell(o *Obs, mx, my *float64) bool {
	me := &o.Self
	g := b.Cfg.Weapons["mortar"].Gravity
	if g <= 0 {
		g = 22
	}
	rad := b.Cfg.Weapons["mortar"].SplashRadius + me.Stats.Radius + 2
	for i := range o.Projectiles {
		p := &o.Projectiles[i]
		if p.Kind != "mortar" {
			continue
		}
		px, py, pz := p.Pos[0], p.Pos[1], p.Pos[2]
		vz := p.Vel[2]
		for t := 0.0; t < 4; t += 0.05 {
			px += p.Vel[0] * 0.05
			py += p.Vel[1] * 0.05
			pz += vz * 0.05
			vz -= g * 0.05
			if b.W.Solid(px, py, pz) {
				d := hyp(px-me.Pos[0], py-me.Pos[1])
				if d < rad && t < 2.5 {
					dx, dy := me.Pos[0]-px, me.Pos[1]-py
					if d < 0.5 {
						dx, dy = -p.Vel[1], p.Vel[0]
						d = hyp(dx, dy)
					}
					dx, dy = dx/d, dy/d

					for _, rot := range []float64{0, 0.7, -0.7, 1.4, -1.4} {
						c, s := math.Cos(rot), math.Sin(rot)
						ex, ey := dx*c-dy*s, dx*s+dy*c
						if b.W.LineWalkable(me.Pos[0], me.Pos[1], me.Pos[0]+ex*6, me.Pos[1]+ey*6) {
							*mx, *my = ex, ey
							return true
						}
					}
				}
				break
			}
		}
	}
	return false
}

func (b *Brain) travel(me *Self, gx, gy float64) (float64, float64) {
	x, y := me.Pos[0], me.Pos[1]
	d := hyp(gx-x, gy-y)
	if d < 0.8 {
		return 0, 0
	}
	unit := func(tx, ty float64) (float64, float64) {
		l := hyp(tx-x, ty-y)
		if l < 1e-6 {
			return 0, 0
		}
		return (tx - x) / l, (ty - y) / l
	}

	if me.Pos[2]-b.W.GroundAt(x, y) > 3 && b.W.Obst[b.W.vidx(x, y)] {
		return unit(gx, gy)
	}
	if d < 45 && b.W.LineWalkable(x, y, gx, gy) {
		return unit(gx, gy)
	}
	if b.path == nil || hyp(gx-b.pathGoal[0], gy-b.pathGoal[1]) > 5 || b.now-b.pathAt > 2.5 {
		b.path, _ = b.W.Path(x, y, gx, gy, 22000)
		b.pathGoal = [2]float64{gx, gy}
		b.pathAt = b.now
		b.pathI = 0
	}
	if len(b.path) == 0 {
		return unit(gx, gy)
	}

	bi, bd := b.pathI, math.Inf(1)
	for i := b.pathI; i < len(b.path) && i < b.pathI+10; i++ {
		px, py := b.W.CellPos(int(b.path[i]))
		if dd := hyp(px-x, py-y); dd < bd {
			bi, bd = i, dd
		}
	}
	b.pathI = bi
	if bd > 12 {
		b.path = nil
		return unit(gx, gy)
	}
	for i := min(bi+14, len(b.path)-1); i > bi; i-- {
		px, py := b.W.CellPos(int(b.path[i]))
		if b.W.LineWalkable(x, y, px, py) {
			if i == len(b.path)-1 && hyp(px-gx, py-gy) < 4 && b.W.LineWalkable(x, y, gx, gy) {
				return unit(gx, gy)
			}
			return unit(px, py)
		}
	}
	if bi+1 < len(b.path) {
		px, py := b.W.CellPos(int(b.path[bi+1]))
		return unit(px, py)
	}
	return unit(gx, gy)
}

// sqHuntOK (sqhunt) reports whether t is a huntable mech in squad mode: seen in the last 2.5 s within SqHunt m,
// our HP > 60% and (our HP x our damage multiplier) / (its HP x its) >= 0.7. The damage multiplier is the cube bonus
// (damage_per_cube, capped at stat_cap) x 1.5 when a laser or lightning is held.
func (b *Brain) sqHuntOK(me *Self, t *Track, hpFrac float64) bool {
	if b.P.SqHunt <= 0 || !b.pracOn || t.Kind != "mech" || b.now-t.Seen > 2.5 || hpFrac <= 0.6 || t.HP <= 0 {
		return false
	}
	if hyp(t.Pos[0]-me.Pos[0], t.Pos[1]-me.Pos[1]) > b.P.SqHunt {
		return false
	}
	mult := func(cubes int, armed bool) float64 {
		c := math.Min(float64(cubes), math.Max(1, float64(b.Cfg.Cubes.StatCap)))
		m := 1 + b.Cfg.Cubes.DamagePerCube*c
		if armed {
			m *= 1.5
		}
		return m
	}
	_, ml := hasWeapon(me.Weapons, "laser")
	_, mg := hasWeapon(me.Weapons, "lightning")
	ours := me.HP * mult(me.Cubes, ml != nil || mg != nil)
	theirs := t.HP * mult(t.Cubes, holds(t.Weapons, "laser") || holds(t.Weapons, "lightning"))
	return ours >= 0.7*theirs
}

func (b *Brain) prefRange(me *Self, t *Track) float64 {
	_, light := hasWeapon(me.Weapons, "lightning")
	_, laser := hasWeapon(me.Weapons, "laser")
	r := b.P.GunRange
	switch {
	case light != nil && me.Energy > 12:
		r = b.P.LightRange
	case laser != nil:
		r = b.P.LaserRange
	}
	if t.Kind == "npc" {
		return 58
	}
	if light == nil {
		if holds(t.Weapons, "lightning") {
			r = math.Max(r, 64)
		}
		if holds(t.Weapons, "mortar") {
			r = math.Max(r, 80)
		}
	}
	if b.P.TPJoin > 0 && b.ffaNow && b.now-t.TPAt < 1.5 {
		r = math.Max(r, t.OppDist+b.P.TPJoin) // tpjoin: stay the farther attacker
	}
	if b.P.SqHunt > 0 && b.pracOn && t.Kind == "mech" && laser == nil && light == nil {
		r = math.Max(18, math.Min(42, t.Dist)) // sqhunt: gun fights from 18-42 m
	}
	if b.P.LCDPunish > 0 && !b.ffaNow && b.lcdWindow(t) && me.HP >= t.HP {
		r = math.Min(r, 50) // lcdpunish: close in while its laser cools down
	}
	if b.P.LightPad > 0 && !b.ffaNow && t.Kind == "mech" && holds(t.Weapons, "lightning") {
		// lightpad: never inside its lightning reach; with a laser, fight it at laser range (kite).
		if laser != nil {
			r = math.Max(r, b.P.LaserRange)
		}
		r = math.Max(r, b.lightReach())
	}
	return r
}

// squadBand (sqband) is the stand-off band [lo, hi] against a mech under squad params: gun 85-125 m, laser
// 110-175 m, each capped 15 m inside the weapon's config range. It returns (rp, rp), i.e. giddy's single range,
// for NPCs, with lightning in use, or when prefRange moved the range away from the plain weapon range.
func (b *Brain) squadBand(me *Self, t *Track, rp float64) (float64, float64) {
	if t.Kind == "npc" {
		return rp, rp
	}
	_, light := hasWeapon(me.Weapons, "lightning")
	_, laser := hasWeapon(me.Weapons, "laser")
	var w *WeaponState
	lo, hi, base := 85.0, 125.0, b.P.GunRange
	switch {
	case light != nil && me.Energy > 12:
		return rp, rp
	case laser != nil:
		w, lo, hi, base = laser, 110, 175, b.P.LaserRange
	default:
		_, w = hasWeapon(me.Weapons, "gun")
	}
	if rp != base {
		return rp, rp
	}
	if w != nil && w.Stats.Range > 0 {
		hi = math.Min(hi, w.Stats.Range-15)
	}
	if hi <= lo {
		return rp, rp
	}
	return lo, hi
}

func (b *Brain) fightMove(o *Obs, tg *Track, seen []*Track, wantLos float64) (float64, float64) {
	me := &o.Self
	x, y := me.Pos[0], me.Pos[1]
	rp := b.prefRange(me, tg)
	lo, hi := rp, rp
	if b.P.SqBand && b.pracOn && !b.finMove {
		lo, hi = b.squadBand(me, tg, rp)
	}
	bx, by := tg.Pos[0]-x, tg.Pos[1]-y
	bd := hyp(bx, by) + 1e-9
	ux, uy := bx/bd, by/bd
	if b.now >= b.strafeUntil {
		if b.P.SqJink && b.pracOn {
			b.strafe = -b.strafe // sqjink: always reverse, every 0.2-0.5 s
			b.strafeUntil = b.now + 0.2 + b.rng.Float64()*0.3
		} else {
			if b.rng.Float64() < 0.6 {
				b.strafe = -b.strafe
			}
			b.strafeUntil = b.now + 0.3 + b.rng.Float64()*0.7
		}
	}
	sx, sy := -uy*b.strafe, ux*b.strafe
	tc := b.center(tg)
	mh := me.Stats.MuzzleHeight
	var cube *known
	for _, c := range b.cubes {
		if d := hyp(c.Pos[0]-x, c.Pos[1]-y); d < 40 && (cube == nil || d < hyp(cube.Pos[0]-x, cube.Pos[1]-y)) && math.Abs(c.Pos[2]-me.Pos[2]) < 4 && !b.cubeSkip(me, c.Pos) {
			cube = c
		}
	}
	best := math.Inf(-1)
	var mx, my float64
	blockedSide := false
	rings := 1
	if b.P.Peek {
		rings = 2
	}
	for k := -1; k < 16*rings; k++ {
		cx, cy, dx, dy, r := x, y, 0.0, 0.0, 0.0
		if k >= 0 {
			a := float64(k%16) * math.Pi / 8
			r = 6 * float64(1+k/16)
			dx, dy = math.Cos(a), math.Sin(a)
			cx, cy = x+dx*r, y+dy*r
			if !b.W.LineWalkable(x, y, cx, cy) || !b.W.Free(cx, cy) {
				if k < 16 && dx*sx+dy*sy > 0.9 {
					blockedSide = true
				}
				continue
			}
		}
		cz := b.W.WalkAt(cx, cy)
		d := hyp(tg.Pos[0]-cx, tg.Pos[1]-cy)
		dev := math.Abs(d - rp)
		if hi > lo { // sqband: no range penalty inside [lo, hi]
			dev = math.Max(0, math.Max(lo-d, d-hi))
		}
		s := -1.1*dev - r*0.4
		if b.kiteOn {
			s += b.P.KiteW * (d - bd) // kitew: back off from a target well inside our range
		}
		los := b.W.Clear(Vec3{cx, cy, cz + mh}, tc)
		if los {
			s += 45 * wantLos
		}
		if b.P.Peek && k >= 0 && k < 16 && (los == (wantLos > 0)) {

			for q := 0; q < 16; q += 2 {
				qx, qy := cx+math.Cos(float64(q)*math.Pi/8)*7, cy+math.Sin(float64(q)*math.Pi/8)*7
				if b.W.Free(qx, qy) && b.W.Clear(Vec3{qx, qy, b.W.WalkAt(qx, qy) + mh}, tc) != los {
					s += 8
					break
				}
			}
		}
		body := Vec3{cx, cy, cz + me.Stats.Height*0.6}
		if b.P.LightPad > 0 && !b.ffaNow {
			// lightpad: keep candidate spots out of other lightning holders' reach.
			lr := b.lightReach()
			for _, e := range seen {
				if e != tg && e.Kind == "mech" && b.now-e.Seen < 2.5 && holds(e.Weapons, "lightning") {
					if de := hyp(e.Pos[0]-cx, e.Pos[1]-cy); de < lr {
						s -= 2 * (lr - de)
					}
				}
			}
		}
		toTg := math.Atan2(tg.Pos[1]-cy, tg.Pos[0]-cx)
		for _, e := range seen {
			if e == tg || hyp(e.Pos[0]-cx, e.Pos[1]-cy) > 170 {
				continue
			}
			if b.W.Clear(b.eye(e), body) {
				s -= 30
				// rearavoid (practice / pre-latch only): aiming at tg from here leaves e in our rear arc (x1.5).
				if b.P.RearAvoid > 0 && !b.ffaNow && e.Seen == b.now &&
					math.Abs(angDiff(math.Atan2(e.Pos[1]-cy, e.Pos[0]-cx), toTg)) > math.Pi/2 {
					s -= b.P.RearAvoid
				}
			}
		}
		s -= b.zonePenalty(&o.Zone, cx, cy)
		if cube != nil {
			s += 1.5 * (hyp(cube.Pos[0]-x, cube.Pos[1]-y) - hyp(cube.Pos[0]-cx, cube.Pos[1]-cy))
		}
		if cl := float64(b.W.Clearance[b.W.Cell(cx, cy)]); cl < 4 {
			s -= (4 - cl) * 2.5
		}
		if wantLos > 0 {
			s += 9 * (dx*sx + dy*sy)
		}
		if k < 0 {
			s -= 8
			if wantLos < 0 && !los {
				s += 14
			}
		}
		if s > best {
			best, mx, my = s, dx, dy
		}
	}
	if blockedSide {
		b.strafe = -b.strafe
		b.strafeUntil = b.now + 0.5
	}
	return mx, my
}

func (b *Brain) coverMove(o *Obs, seen []*Track, mx, my *float64) bool {
	me := &o.Self
	x, y := me.Pos[0], me.Pos[1]
	var threats []*Track
	for _, t := range seen {
		if hyp(t.Pos[0]-x, t.Pos[1]-y) < 230 {
			threats = append(threats, t)
		}
	}
	if len(threats) == 0 {
		return false
	}
	if len(threats) > 4 {
		sort.Slice(threats, func(i, j int) bool {
			return hyp(threats[i].Pos[0]-x, threats[i].Pos[1]-y) < hyp(threats[j].Pos[0]-x, threats[j].Pos[1]-y)
		})
		threats = threats[:4]
	}
	hidden := func(cx, cy float64) bool {
		cz := b.W.WalkAt(cx, cy)
		for _, t := range threats {
			e := b.eye(t)
			if b.W.Clear(e, Vec3{cx, cy, cz + me.Stats.Height}) || b.W.Clear(e, Vec3{cx, cy, cz + me.Stats.Height*0.5}) {
				return false
			}
		}
		return true
	}
	if b.coverOK && b.now-b.coverAt < 0.4 {
		*mx, *my = b.travelCover(me)
		return true
	}
	b.coverAt = b.now
	if hidden(x, y) && b.zonePenalty(&o.Zone, x, y) == 0 {
		b.coverOK = true
		b.coverGoal = [2]float64{x, y}
		*mx, *my = 0, 0
		return true
	}

	w := b.W
	s := w.NearestFree(w.Cell(x, y))
	if s < 0 {
		return false
	}
	w.gen++
	gen := w.gen
	queue := []int32{int32(s)}
	w.stamp[s] = gen
	w.g[s] = 0
	bestC, bestS := -1, math.Inf(1)
	checked := 0
	maxChecks, maxSteps := 350, 50.0
	if b.coverChecks > 0 {
		maxChecks, maxSteps = b.coverChecks, float64(b.coverSteps) // cornercover: wider search
	}
	for h := 0; h < len(queue) && checked < maxChecks; h++ {
		c := int(queue[h])
		ci, cj := c%w.NNX, c/w.NNX
		steps := float64(w.g[c])
		if steps > maxSteps || (bestC >= 0 && steps*2 > bestS+30) {
			break
		}
		if (ci+cj)%2 == 0 && steps > 0 {
			cx, cy := w.CellPos(c)
			checked++
			if hidden(cx, cy) {
				sc := steps*2 + b.zonePenalty(&o.Zone, cx, cy)
				near := math.Inf(1)
				for _, t := range threats {
					near = math.Min(near, hyp(t.Pos[0]-cx, t.Pos[1]-cy))
				}
				sc -= math.Min(near, 80) * 0.25
				if sc < bestS {
					bestC, bestS = c, sc
				}
			}
		}
		for k, o2 := range nbrs {
			i, j := ci+o2[0], cj+o2[1]
			if i < 0 || j < 0 || i >= w.NNX || j >= w.NNY {
				continue
			}
			nb := j*w.NNX + i
			if w.stamp[nb] == gen || !w.canStep(c, nb, k >= 4) {
				continue
			}
			if k >= 4 && (w.Blocked[cj*w.NNX+i] || w.Blocked[j*w.NNX+ci]) {
				continue
			}
			w.stamp[nb] = gen
			w.g[nb] = w.g[c] + 1
			queue = append(queue, int32(nb))
		}
	}
	if bestC < 0 {
		b.coverOK = false
		return false
	}
	gx, gy := w.CellPos(bestC)
	b.coverOK = true
	b.coverGoal = [2]float64{gx, gy}
	*mx, *my = b.travelCover(me)
	return true
}

func (b *Brain) travelCover(me *Self) (float64, float64) {
	if hyp(b.coverGoal[0]-me.Pos[0], b.coverGoal[1]-me.Pos[1]) < 1.5 {
		return 0, 0
	}
	return b.travel(me, b.coverGoal[0], b.coverGoal[1])
}

type lootChoice struct {
	kind  string
	id    int
	pos   Vec3
	value float64
}

func (b *Brain) chooseLoot(o *Obs, hpFrac float64, danger bool) *lootChoice {
	me := &o.Self
	x, y := me.Pos[0], me.Pos[1]
	z := &o.Zone
	now := b.now
	cur := b.curZone(z)
	var best *lootChoice
	bestU := 0.0
	econ := b.P.Econ
	// earlyspace: before EarlySpaceT, loot costs extra "metres" for nearby mechs; a weapon pickup that is the only
	// wanted one within 90 m is exempt (never block arming).
	esOn := b.P.EarlySpace > 0 && now < b.earlySpaceT()
	soleWeapon := -1
	if esOn {
		n := 0
		for _, p := range b.pickups {
			if b.wantPickup(me, p.Kind) && hyp(p.Pos[0]-x, p.Pos[1]-y) < 90 {
				n++
				soleWeapon = p.ID
			}
		}
		if n != 1 {
			soleWeapon = -1
		}
	}
	lootIn := 0.0 // sqloot: loot deep inside the next circle only this close (0 = no limit)
	if b.P.SqLootIn > 0 && b.pracOn && z.NextRadius > 0 {
		lootIn = b.P.SqLootIn
	}
	consider := func(kind string, id int, pos Vec3, value float64) {
		if until, ok := b.bad[id]; ok && until > now {
			return
		}
		d := hyp(pos[0]-x, pos[1]-y)
		if d > 260 {
			return
		}
		if lootIn > 0 && d > lootIn && hyp(pos[0]-z.NextCenter[0], pos[1]-z.NextCenter[1]) < 0.81*z.NextRadius {
			if p := b.pickups[id]; kind != "pickup" || p == nil || (p.Kind != "laser" && p.Kind != "lightning") {
				return
			}
		}
		if hyp(pos[0]-cur.X, pos[1]-cur.Y) > cur.R-6 {
			return
		}

		if dl := b.zoneDeadline(z, pos[0], pos[1], 6); now+d*1.4/me.Stats.WalkSpeed+5 > dl {
			return
		}

		if pos[2]-b.W.GroundAt(pos[0], pos[1]) > 3.2 {
			return
		}
		if nf := b.W.NearestFree(b.W.Cell(pos[0], pos[1])); nf < 0 || b.W.Comp[nf] != b.W.MainComp {
			return
		}
		es := 0.0
		if esOn && !(kind == "pickup" && id == soleWeapon) {
			es = b.earlySpaceCost(pos[0], pos[1])
		}
		u := value / (d + 18 + es)
		if id == b.lootID {
			u *= 1.35
		}
		if econ {
			u = value - 0.6*(d+es)
			if id == b.lootID {
				u += 30
			}
		}
		if u > bestU {
			bestU = u
			best = &lootChoice{kind, id, pos, value}
		}
	}
	if econ {
		mechNear := false
		for _, t := range b.tracks {
			if t.Seen == now {
				mechNear = true
			}
		}
		hurt := hpFrac <= 0.45
		nearOnly := false
		for _, t := range b.tracks {
			if b.P.EconSafe > 0 && now-t.Seen < b.P.EconSafe {
				nearOnly = true
			}
		}
		inner := consider
		consider = func(kind string, id int, pos Vec3, value float64) {
			if nearOnly && hyp(pos[0]-x, pos[1]-y) > 60 {
				return
			}
			inner(kind, id, pos, value)
		}
		for _, p := range b.pickups {
			if !b.wantPickup(me, p.Kind) || hurt {
				continue
			}
			v := 150.0
			if len(me.Weapons) > 1 && me.Weapons[1] == nil {
				v = 260
			}
			if p.Kind == "laser" {
				v = math.Max(v, 270) + 20
			}
			consider("pickup", p.ID, p.Pos, v)
		}
		for _, c := range b.cubes {
			if (!hurt || hyp(c.Pos[0]-x, c.Pos[1]-y) < 35) && !b.cubeSkip(me, c.Pos) {
				consider("cube", c.ID, c.Pos, 110)
			}
		}
		for _, c := range b.crates {
			if (!hurt || hyp(c.Pos[0]-x, c.Pos[1]-y) < 35) && !b.cubeSkip(me, c.Pos) {
				consider("crate", c.ID, c.Pos, 105)
			}
		}
		if b.P.FarmNPC && hpFrac > 0.6 && !mechNear {
			for _, n := range b.npcs {
				v := 170.0
				if now-n.Seen > 12 {
					v = 155
				}
				consider("npc", n.ID, n.Pos, v)
			}
		}
		for i := range o.Drops {
			d := &o.Drops[i]
			if !hurt && hyp(d.Pos[0]-x, d.Pos[1]-y)/me.Stats.WalkSpeed < d.LandsIn+25 && b.dropsOK(o, d) {
				consider("drop", d.ID, d.Pos, 170)
			}
		}
		if best == nil {
			b.lootID = -1
			return nil
		}
		if best.id != b.lootID {
			b.lootID = best.id
			b.lootSince = now
		} else if lim := 30.0; now-b.lootSince > lim || (best.kind == "crate" && now-b.lootSince > lim-6 && hyp(best.pos[0]-x, best.pos[1]-y) < 50) {
			b.bad[best.id] = now + 60
			b.lootID = -1
			return nil
		}
		return best
	}
	armOn := b.armActive(me)
	armFirst := armOn && b.P.ArmFirst && now < dflt(b.base.PracAt, 20)
	armCand := false
	armR := 0.0
	if armOn {
		armR = dflt(b.P.ArmRFar, 160)
		for _, t := range b.tracks {
			if !armFirst && now-t.Seen < 5 && hyp(t.Pos[0]-x, t.Pos[1]-y) < 150 {
				armR = dflt(b.P.ArmR, 120)
				break
			}
		}
	}
	for _, p := range b.pickups {
		if armOn && (p.Kind == "laser" || p.Kind == "lightning") {
			if !b.wantPickup(me, p.Kind) || hyp(p.Pos[0]-x, p.Pos[1]-y) >= armR {
				continue
			}
			blocked := false
			for _, t := range b.tracks {
				if now-t.Seen < 5 && segDist(t.Pos[0], t.Pos[1], x, y, p.Pos[0], p.Pos[1]) < dflt(b.P.ArmRoute, 60) {
					blocked = true
					break
				}
			}
			if !blocked {
				armCand = true
				v := dflt(b.P.ArmV, 400)
				if armFirst {
					d := hyp(p.Pos[0]-x, p.Pos[1]-y)
					if now+d*1.4/me.Stats.WalkSpeed+8 > b.zoneDeadline(z, p.Pos[0], p.Pos[1], 6) {
						continue // armfirst: the zone gets there first
					}
					v /= 1 + float64(b.losCount(p.Pos)) // armfirst: prefer the pickup fewer mechs can see
					for _, t := range b.tracks {
						if t.Kind == "mech" && now-t.Seen < 5 && hyp(t.Pos[0]-p.Pos[0], t.Pos[1]-p.Pos[1]) < 0.6*d {
							v *= 0.3 // armfirst: contested, another mech is much closer to it
							break
						}
					}
				}
				consider("pickup", p.ID, p.Pos, v)
			}
			continue
		}
		rad := b.P.LootRadius
		seek := p.Kind == "laser" && b.P.LaserSeek > rad
		if seek {
			rad = b.P.LaserSeek
		}
		if b.wantPickup(me, p.Kind) && hyp(p.Pos[0]-x, p.Pos[1]-y) < rad {
			v := 60.0
			if len(me.Weapons) > 1 && me.Weapons[1] == nil {
				v = 130
			}
			if seek {
				v = math.Max(v, 150)
			}
			if b.P.KeepLaser && b.ffaNow && p.Kind == "laser" {
				if _, l := hasWeapon(me.Weapons, "lightning"); l != nil {
					v = math.Max(v, 150) // keeplaser: swap held lightning for a laser in reach
				}
			}
			consider("pickup", p.ID, p.Pos, v)
		}
	}
	cubeV := 75.0
	if me.Cubes >= b.cubeCap() {
		cubeV = 25
	} else if b.P.CubePush > 0 && b.ffaNow && now < b.P.CubePush {
		cubeV *= dflt(b.P.CubePushK, 2.5) // cubepush (crates use cubeV*0.6 below)
	}
	weaponFirst := armFirst && armCand // armfirst: no cube, crate or drone detours while a weapon is on offer
	for _, c := range b.cubes {
		if !weaponFirst && hyp(c.Pos[0]-x, c.Pos[1]-y) < math.Max(60, b.P.LootRadius*0.7) && !b.cubeSkip(me, c.Pos) {
			consider("cube", c.ID, c.Pos, cubeV)
		}
	}
	farmHP := 0.7
	if b.P.FarmHP > 0 {
		farmHP = b.P.FarmHP
	}
	if hpFrac > farmHP && !danger && now < b.P.CrateUntil && !weaponFirst {
		for _, c := range b.crates {
			if d := hyp(c.Pos[0]-x, c.Pos[1]-y); armOn && !armCand && d < dflt(b.P.ArmCrate, 60) && !b.cubeSkip(me, c.Pos) {
				consider("crate", c.ID, c.Pos, 150) // arm: crate fallback (a crate drops a weapon half the time)
			} else if d < b.P.LootRadius && !b.cubeSkip(me, c.Pos) {
				consider("crate", c.ID, c.Pos, cubeV*0.6)
			}
		}
		if b.P.FarmNPC && me.Cubes < b.cubeCap() {
			for _, n := range b.npcs {
				if now-n.Seen < 12 && hyp(n.Pos[0]-x, n.Pos[1]-y) < b.P.LootRadius {
					consider("npc", n.ID, n.Pos, 85)
				}
			}
		}
	}
	for i := range o.Drops {
		d := &o.Drops[i]
		dist := hyp(d.Pos[0]-x, d.Pos[1]-y)
		if dist/me.Stats.WalkSpeed < d.LandsIn+25 && !danger && b.dropsOK(o, d) {
			consider("drop", d.ID, d.Pos, 260)
		}
	}
	if best == nil {
		b.lootID = -1
		return nil
	}
	if best.id != b.lootID {
		b.lootID = best.id
		b.lootSince = now
	} else if now-b.lootSince > 30 {
		b.bad[best.id] = now + 60
		b.lootID = -1
		return nil
	}
	return best
}

// dropsOK says whether chooseLoot may consider supply drop d. UseDrops (giddy's flag, any mode) keeps giddy's
// behaviour. FFADrops (ffadrops=<max m>) allows it only after a proven free-for-all (ffaOn), and only when:
//   - it is one of the first two drops of the match and at most FFADrops m away;
//   - no other mech is capturing it (holder), and at least DropAlive mechs are alive (if set);
//   - always: no mech has hit us in the last 3 s and no mech seen this tick is within 100 m; below 50% HP, no
//     mech seen within 200 m in the last 5 s (a capture means standing still for 4 s);
//   - to commit, calm: nothing hit us in the last 5 s, no mech is visible this tick, and none was seen within
//     150 m in the last 5 s. We stay committed only while the drop is our loot target and passed on the previous
//     decision; any decision spent in another mode (dodge, unstuck, fight...) ends the commitment, so resuming
//     needs the calm test again.
//
// A committed drop that fails is barred for 8 s (b.bad), so we do not walk back as soon as the threat steps out
// of view. Once committed, NPC fire does not abort: drones do not contest a drop and only chase within their leash.
func (b *Brain) dropsOK(o *Obs, d *Drop) bool {
	if b.P.UseDrops {
		return true
	}
	if b.P.FFADrops <= 0 || !b.ffaNow {
		return false
	}
	if d.Holder != nil && *d.Holder != o.Self.ID {
		return false
	}
	if b.P.DropAlive > 0 && o.Alive < b.P.DropAlive {
		return false
	}
	if hyp(d.Pos[0]-o.Self.Pos[0], d.Pos[1]-o.Self.Pos[1]) > b.P.FFADrops {
		return false
	}
	rank := -1
	for i, id := range b.dropOrder {
		if id == d.ID {
			rank = i
		}
	}
	if rank < 0 || rank > 1 {
		return false
	}
	committed := b.lootID == d.ID && b.now-b.dropOKAt < 0.15
	low := o.Self.HP < 0.5*math.Max(1, o.Self.MaxHP)
	if !committed && b.now-b.lastHurt < 5 {
		return false
	}
	for _, t := range b.tracks {
		if t.Kind != "mech" {
			continue
		}
		threat := b.now-t.HitMe < 3 || (t.Seen == b.now && t.Dist < 100) || (low && b.now-t.Seen < 5 && t.Dist < 200)
		calm := t.Seen != b.now && !(b.now-t.Seen < 5 && t.Dist < 150)
		if threat || (!committed && !calm) {
			if committed {
				b.bad[d.ID] = b.now + 8
			}
			return false
		}
	}
	if b.lootID == d.ID || committed {
		b.dropOKAt = b.now
	}
	return true
}

// earlyLosing (earlyretreat) reports whether a mech that hit us in the last 1.5 s is winning the trade: it has
// more than 1.15x our HP and an HP fraction above ours + 0.12 while we are below 80%, or we are gun-only and it
// holds a laser or lightning.
func (b *Brain) earlyLosing(me *Self, hpFrac float64, gunOnly bool) bool {
	for _, t := range b.tracks {
		if t.Kind != "mech" || b.now-t.HitMe >= 1.5 {
			continue
		}
		if gunOnly && (holds(t.Weapons, "laser") || holds(t.Weapons, "lightning")) {
			return true
		}
		tf := t.HP / math.Max(1, t.MaxHP)
		if hpFrac < 0.8 && t.HP > 1.15*me.HP && tf > hpFrac+0.12 {
			return true
		}
	}
	return false
}

// noteLaserShots (lcdpunish) records when a tracked laser holder fired: a laser beam it fired (by id), or a laser
// sound (near/mid) within 0.35 rad of the bearing of exactly one visible laser holder.
func (b *Brain) noteLaserShots(o *Obs) {
	for _, bm := range o.Beams {
		if bm.Kind == "laser" {
			if t := b.tracks[bm.By]; t != nil {
				t.LaserAt = math.Max(t.LaserAt, b.now-bm.Age)
			}
		}
	}
	me := &o.Self
	for _, snd := range o.Sounds {
		if snd.Kind != "laser" || snd.Dist == "far" {
			continue
		}
		var who *Track
		n := 0
		for _, t := range b.tracks {
			if t.Kind == "mech" && t.Seen == b.now && holds(t.Weapons, "laser") &&
				math.Abs(angDiff(math.Atan2(t.Pos[1]-me.Pos[1], t.Pos[0]-me.Pos[0]), snd.Bearing)) < 0.35 {
				who, n = t, n+1
			}
		}
		if n == 1 {
			who.LaserAt = math.Max(who.LaserAt, b.now-snd.Age)
		}
	}
}

// lcdWindow (lcdpunish) reports whether t fired its laser less than LCDPunish s ago (capped at the configured
// laser cooldown), so it cannot fire it again yet.
func (b *Brain) lcdWindow(t *Track) bool {
	w := b.P.LCDPunish
	if cd := b.Cfg.Weapons["laser"].Cooldown; cd > 0 {
		w = math.Min(w, cd)
	}
	return b.now-t.LaserAt < w
}

// dmgMult (smartfire) is our damage multiplier from cubes (capped at cubes.stat_cap) and, when we stand in t's
// rear arc (showsBack, with its 0.1 rad margin), mech.rear_mult.
func (b *Brain) dmgMult(me *Self, t *Track, x, y float64) float64 {
	m := 1 + b.Cfg.Cubes.DamagePerCube*float64(min(me.Cubes, b.Cfg.Cubes.StatCap))
	if b.showsBack(t, x, y) {
		m *= b.rearMult()
	}
	return m
}

// laserDmg (smartfire) is the damage our laser w deals t if released at the given charge (seconds).
func (b *Brain) laserDmg(me *Self, w *WeaponState, t *Track, charge, x, y float64) float64 {
	full := math.Max(w.Stats.Charge, 1e-6)
	d := w.Stats.Damage * (w.Stats.MinFrac + (1-w.Stats.MinFrac)*math.Min(charge, full)/full) * b.dmgMult(me, t, x, y)
	if holds(t.Weapons, "mortar") {
		d *= 1 + w.Stats.Cookoff
	}
	return d
}

// canKill (smartfire) reports whether our next burst kills t at the SmartFire safety factor: a ready laser
// (cooldown <= 0.1 s, in range) at full charge, or a gun burst of at most 1 s within our ammo at <= 60 m.
func (b *Brain) canKill(me *Self, t *Track, gun, laser *WeaponState, x, y float64) bool {
	k := b.P.SmartFire
	if laser != nil && laser.Cooldown <= 0.1 && t.Dist <= laser.Stats.Range*0.97 &&
		b.laserDmg(me, laser, t, laser.Stats.Charge, x, y)*k >= t.HP {
		return true
	}
	if gun != nil && gun.Reload <= 0 && t.Dist <= math.Min(60, gun.Stats.Range*0.96) && gun.Stats.Damage > 0 {
		per := gun.Stats.Damage * b.dmgMult(me, t, x, y) * k
		n := math.Ceil(t.HP / per)
		if n <= float64(gun.Ammo) && n*gun.Stats.Cooldown <= 1.0 {
			return true
		}
	}
	return false
}

// rimSpot (pracrim) is the point PracRim x NextRadius from the next circle's centre in our direction (east if we
// stand on the centre).
func (b *Brain) rimSpot(z *Zone, x, y float64) (float64, float64) {
	dx, dy := x-z.NextCenter[0], y-z.NextCenter[1]
	d := hyp(dx, dy)
	if d < 1 {
		dx, dy, d = 1, 0, 1
	}
	r := b.P.PracRim * z.NextRadius
	return z.NextCenter[0] + dx/d*r, z.NextCenter[1] + dy/d*r
}

// squadRimSpot (sqrim) is where idle squad play waits: SqRim x the next circle's radius from its centre while the
// radius is >= 200 m (0.6 x below 200 m, 0.35 x below 50 m), at least 8 m inside the circle, on our side of the
// centre (east if we stand on it), turned along the rim away from the mechs seen in the last 20 s. Each such mech
// pushes us by up to 0.5 rad, more when it is close to our bearing and far from the centre; the total push is
// clamped to +-0.5 rad, so we drift while mechs are about and hold still once they are gone.
func (b *Brain) squadRimSpot(z *Zone, x, y float64) (float64, float64) {
	cx, cy, R := z.NextCenter[0], z.NextCenter[1], z.NextRadius
	f := b.P.SqRim
	switch {
	case R < 50:
		f = 0.35
	case R < 200:
		f = 0.6
	}
	r := math.Max(0, math.Min(f*R, R-8))
	th := 0.0
	if hyp(x-cx, y-cy) >= 1 {
		th = math.Atan2(y-cy, x-cx)
	}
	push := 0.0
	for _, t := range b.recentMechs(x, y, 20) { // sorted, so the sum is reproducible
		rho := hyp(t.Pos[0]-cx, t.Pos[1]-cy)
		if rho < 1 || r < 1 {
			continue
		}
		dl := angDiff(th, math.Atan2(t.Pos[1]-cy, t.Pos[0]-cx))
		w := 0.5 * (1 - math.Abs(dl)/math.Pi) * math.Min(1, rho/r)
		if dl < 0 {
			w = -w
		}
		push += w
	}
	th += clamp(push, -0.5, 0.5)
	return cx + math.Cos(th)*r, cy + math.Sin(th)*r
}

// regenWin (regenlock) is one mech's regen window as far as our hits show it: when the window (re)started, our
// last hit on it and the damage we landed since the start.
type regenWin struct {
	start, last, dmg float64
}

// regenDelay (regenlock) is mech.regen_delay from hello.config (5 s if missing).
func (b *Brain) regenDelay() float64 {
	return dflt(b.Cfg.Mech.RegenDelay, 5)
}

// noteRegenHits (regenlock) folds this tick's hit events on mechs into the per-mech regen windows. A hit after the
// window has lapsed (regen_delay since our last hit) starts a new window.
func (b *Brain) noteRegenHits(o *Obs) {
	if b.rlWin == nil {
		b.rlWin = map[int]*regenWin{}
	}
	for i := range o.Events {
		e := &o.Events[i]
		if e.Type != "hit" || e.Amount <= 0 || b.tracks[e.Target] == nil {
			continue
		}
		w := b.rlWin[e.Target]
		if w == nil || o.T-w.last >= b.regenDelay() {
			w = &regenWin{start: o.T}
			b.rlWin[e.Target] = w
		}
		w.last, w.dmg = o.T, w.dmg+e.Amount
	}
	for id, w := range b.rlWin {
		if o.T-w.last >= b.regenDelay() || b.tracks[id] == nil {
			delete(b.rlWin, id)
		}
	}
}

// regenLive (regenlock) returns mech id's regen window if our last hit on it is less than regen_delay ago.
func (b *Brain) regenLive(id int) *regenWin {
	if w := b.rlWin[id]; w != nil && b.now-w.last < b.regenDelay() {
		return w
	}
	return nil
}

// regenChase (regenlock) picks the mech to walk after: a live regen window, last seen out of sight or out of our
// target range, reachable to within our weapon reach before the window ends, with our HP >= RegenLockHP (0.4) of
// max and no other mech seen in the last 2.5 s within 60 m. The most recently hit one wins (ties: lower id).
func (b *Brain) regenChase(me *Self, hpFrac float64) *Track {
	if hpFrac < dflt(b.P.RegenLockHP, 0.4) {
		return nil
	}
	var best *Track
	bestLast := math.Inf(-1)
	for id, w := range b.rlWin {
		t := b.tracks[id]
		if t == nil || b.regenLive(id) == nil {
			continue
		}
		left := b.regenDelay() - (b.now - w.last)
		d := hyp(t.Pos[0]-me.Pos[0], t.Pos[1]-me.Pos[1])
		if math.Max(0, d-b.reach(me))/math.Max(1, me.Stats.WalkSpeed) >= left {
			continue
		}
		if w.last > bestLast || (w.last == bestLast && best != nil && id < best.ID) {
			best, bestLast = t, w.last
		}
	}
	if best == nil {
		return nil
	}
	for _, t := range b.tracks {
		if t != best && b.now-t.Seen < 2.5 && hyp(t.Pos[0]-me.Pos[0], t.Pos[1]-me.Pos[1]) < 60 {
			return nil
		}
	}
	return best
}

// lightReach (lightpad) is the distance we keep from lightning holders: weapons.lightning.range (45 m if
// missing) plus LightPad.
// threatTau is threatfocus's decay time constant (s).
const threatTau = 6.0

// noteThreat adds amount to attacker id's decayed damage (threatfocus).
func (b *Brain) noteThreat(id int, amount, now float64) {
	if b.thrDmg == nil {
		b.thrDmg = map[int][2]float64{}
	}
	r := b.thrDmg[id]
	b.thrDmg[id] = [2]float64{r[0]*math.Exp(-(now-r[1])/threatTau) + amount, now}
}

// threatRate is attacker id's recent damage rate on us in HP/s (decayed damage / threatTau; threatfocus).
func (b *Brain) threatRate(id int) float64 {
	r, ok := b.thrDmg[id]
	if !ok {
		return 0
	}
	return r[0] * math.Exp(-(b.now-r[1])/threatTau) / threatTau
}

func (b *Brain) lightReach() float64 {
	lr := b.Cfg.Weapons["lightning"].Range
	if lr <= 0 {
		lr = 45
	}
	return lr + b.P.LightPad
}

// cubeSkip (cubestop) says whether to skip a detour to the cube or crate at pos: only when CubeStop is on, the
// match is a proven FFA (ffaOn) and we hold at least cubes.stat_cap cubes, unless the next upgrade threshold is at
// most 2 cubes away and pos is within 40 m.
func (b *Brain) cubeSkip(me *Self, pos Vec3) bool {
	if !b.P.CubeStop || !b.ffaNow {
		return false
	}
	capC := b.Cfg.Cubes.StatCap
	if capC <= 0 {
		capC = 10
	}
	if me.Cubes < capC {
		return false
	}
	for _, th := range b.Cfg.Cubes.UpgradeThresholds {
		if th > me.Cubes {
			if th-me.Cubes <= 2 && hyp(pos[0]-me.Pos[0], pos[1]-me.Pos[1]) <= 40 {
				return false
			}
			break
		}
	}
	return true
}

func (b *Brain) exploreMove(o *Obs) (float64, float64) {
	me := &o.Self
	x, y := me.Pos[0], me.Pos[1]
	z := &o.Zone
	if b.now > b.exploreUntil || hyp(b.explore[0]-x, b.explore[1]-y) < 6 {
		for try := 0; try < 20; try++ {
			a := b.rng.Float64() * 2 * math.Pi
			r := math.Sqrt(b.rng.Float64()) * z.NextRadius * 0.8
			px, py := z.NextCenter[0]+math.Cos(a)*r, z.NextCenter[1]+math.Sin(a)*r
			if !b.W.InBounds(px, py) || !b.W.Free(px, py) {
				continue
			}
			if d := hyp(px-x, py-y); d < 50 || d > 220 {
				continue
			}
			b.explore = [2]float64{px, py}
			b.exploreUntil = b.now + 30
			break
		}
	}
	if b.now > b.exploreUntil {
		return 0, 0
	}
	return b.travel(me, b.explore[0], b.explore[1])
}

func (b *Brain) homeCost(o *Obs, px, py float64) float64 {
	z := &o.Zone
	crowd := 0.0
	crowdR := 80.0
	if b.ffaNow && b.P.CrowdR > 0 {
		crowdR = b.P.CrowdR
	}
	for _, t := range b.tracks {
		if b.now-t.Seen < 20 {
			crowd += math.Max(0, crowdR-hyp(t.Pos[0]-px, t.Pos[1]-py)) / crowdR
		}
	}
	for i := range o.Drops {
		crowd += math.Max(0, 60-hyp(o.Drops[i].Pos[0]-px, o.Drops[i].Pos[1]-py)) / 60
	}
	edge := hyp(px-z.NextCenter[0], py-z.NextCenter[1]) / math.Max(30, z.NextRadius)
	c := b.P.HomeExpo*b.W.Exposure(px, py) + 0.25*hyp(px-o.Self.Pos[0], py-o.Self.Pos[1]) + 40*crowd + 10*edge
	if b.ffaNow && b.P.HomeLOS > 0 {
		c += b.homeLOSCost(o, px, py) // homelos
	}
	if b.P.EarlySpace > 0 && b.now < b.earlySpaceT() {
		c += b.earlySpaceCost(px, py) // earlyspace (all modes, first 20 s)
	}
	return c
}

// homeLOSCost (homelos) is the sightline penalty for standing at (px, py): for each of up to 4 recently seen
// mechs whose eye sees our eye height there, HomeLOS at 85-200 m and 5 x HomeLOS within 85 m, capped at 3 near
// mechs' worth (15 x HomeLOS) so it biases the home choice rather than dominating it.
func (b *Brain) homeLOSCost(o *Obs, px, py float64) float64 {
	spot := Vec3{px, py, b.W.WalkAt(px, py) + o.Self.Stats.EyeHeight}
	c := 0.0
	for _, e := range b.homeEyes(o) {
		d := hyp(e[0]-px, e[1]-py)
		if d > 200 || !b.W.Clear(e, spot) {
			continue
		}
		if d < 85 {
			c += 5 * b.P.HomeLOS
		} else {
			c += b.P.HomeLOS
		}
	}
	return math.Min(c, 15*b.P.HomeLOS)
}

// homeEyes (homelos) returns the eyes of the 4 mechs seen in the last 20 s nearest us, cached per tick, which
// caps a home re-choice at 4 LOS checks per candidate (40 + 1 candidates, 48 + 1 in holdHome).
func (b *Brain) homeEyes(o *Obs) []Vec3 {
	if b.losTick != o.Tick+1 {
		b.losTick = o.Tick + 1
		b.losEyes = b.losEyes[:0]
		for _, t := range b.recentMechs(o.Self.Pos[0], o.Self.Pos[1], 20) {
			if len(b.losEyes) == 4 {
				break
			}
			b.losEyes = append(b.losEyes, b.eye(t))
		}
	}
	return b.losEyes
}

// homeSeen (homelos) reports that a mech seen this tick (one of the 4 nearest the home spot, within 200 m) has
// gained line of sight to our eye height at the current home, so home/holdHome re-choose at once. A gain right
// after a re-choice (within 1 s) waits a tick, so a home that every spot's sightline covers is not re-chosen
// every tick.
func (b *Brain) homeSeen(o *Obs) bool {
	if !b.ffaNow || b.P.HomeLOS <= 0 || !b.homeSet {
		b.homeVis = false
		return false
	}
	hx, hy := b.homeXY[0], b.homeXY[1]
	spot := Vec3{hx, hy, b.W.WalkAt(hx, hy) + o.Self.Stats.EyeHeight}
	vis := false
	for i, t := range b.recentMechs(hx, hy, 0) {
		if i == 4 || hyp(t.Pos[0]-hx, t.Pos[1]-hy) > 200 {
			break
		}
		if b.W.Clear(b.eye(t), spot) {
			vis = true
			break
		}
	}
	if vis && !b.homeVis && b.now-b.homeAt <= 1 {
		return false
	}
	gained := vis && !b.homeVis
	b.homeVis = vis
	return gained
}

// earlySpaceT (earlyspace) is the end of the early window: giddy's practice/squad switch time (20 s).
func (b *Brain) earlySpaceT() float64 {
	return 20
}

// earlySpaceCost (earlyspace) is EarlySpace x (mechs seen in the last 5 s within 85 m of the spot + closeness of
// the nearest such mech, (120 - min(d, 120)) / 120). Real showdowns: at 20 s, distance to the nearest mech goes
// with points (rho +0.24) and enemies in LOS within 85 m against them (rho -0.22) (Data/Winner autopsies.md).
func (b *Brain) earlySpaceCost(px, py float64) float64 {
	n, near := 0.0, math.Inf(1)
	for _, t := range b.tracks {
		if b.now-t.Seen > 5 {
			continue
		}
		d := hyp(t.Pos[0]-px, t.Pos[1]-py)
		if d < 85 {
			n++
		}
		near = math.Min(near, d)
	}
	if math.IsInf(near, 1) {
		return 0
	}
	r := 120.0
	if b.P.EarlySpaceR > 0 {
		r = b.P.EarlySpaceR
	}
	return b.P.EarlySpace * (n + (r-math.Min(near, r))/r)
}

// earlyKeep (earlyspace) sets a walk directly away from the nearest mech seen in the last 5 s when it is closer
// than EarlySpaceKeep (default 150 m); false when no such mech or the rule is off.
func (b *Brain) earlyKeep(me *Self, mx, my *float64) bool {
	keep := 150.0
	if b.P.EarlySpaceKeep < 0 {
		return false
	}
	if b.P.EarlySpaceKeep > 0 {
		keep = b.P.EarlySpaceKeep
	}
	x, y := me.Pos[0], me.Pos[1]
	var near *Track
	nd := keep
	for _, t := range b.tracks {
		if b.now-t.Seen > 5 {
			continue
		}
		if d := hyp(t.Pos[0]-x, t.Pos[1]-y); d < nd {
			near, nd = t, d
		}
	}
	if near == nil {
		return false
	}
	ax, ay := x-near.Pos[0], y-near.Pos[1]
	l := hyp(ax, ay) + 1e-9
	gx, gy := x+ax/l*30, y+ay/l*30
	if !b.W.InBounds(gx, gy) {
		return false
	}
	*mx, *my = b.travel(me, gx, gy)
	return *mx != 0 || *my != 0
}

func (b *Brain) home(o *Obs) (float64, float64, bool) {
	z := &o.Zone
	force := b.homeSeen(o) // homelos: a fresh sightline onto home re-chooses at once (false when off)
	if b.holdingOut(z, o.Alive) {
		return b.holdHome(o, force)
	}
	out := !b.homeSet || hyp(b.homeXY[0]-z.NextCenter[0], b.homeXY[1]-z.NextCenter[1]) > z.NextRadius*0.8
	rechoose := 12.0
	if b.P.EarlySpace > 0 && b.now < b.earlySpaceT() {
		rechoose = 3 // earlyspace
	}
	if out || b.now-b.homeAt > rechoose || force {
		b.homeAt = b.now
		best, bestC := [2]float64{}, math.Inf(1)
		for k := 0; k < 40; k++ {
			a := b.rng.Float64() * 2 * math.Pi
			frac := 0.7
			if b.ffaNow && b.P.HomeFrac > 0 {
				frac = b.P.HomeFrac
			}
			r := math.Sqrt(b.rng.Float64()) * z.NextRadius * frac
			px, py := z.NextCenter[0]+math.Cos(a)*r, z.NextCenter[1]+math.Sin(a)*r
			if !b.W.InBounds(px, py) || !b.W.Reachable(px, py) {
				continue
			}
			if c := b.homeCost(o, px, py); c < bestC {
				best, bestC = [2]float64{px, py}, c
			}
		}
		if !math.IsInf(bestC, 1) && (out || bestC < b.homeCost(o, b.homeXY[0], b.homeXY[1])-15) {
			b.homeXY, b.homeSet = best, true
		}
	}
	return b.homeXY[0], b.homeXY[1], b.homeSet
}

func (b *Brain) hideFromBearing(o *Obs, seen []*Track, mx, my *float64) bool {
	me := &o.Self
	ghost := &Track{}
	ghost.Pos = Vec3{me.Pos[0] + math.Cos(b.hurtBearing)*80, me.Pos[1] + math.Sin(b.hurtBearing)*80, me.Pos[2]}
	return b.coverMove(o, append([]*Track{ghost}, seen...), mx, my)
}

// recentMechs returns the mech tracks seen within the last s seconds (at their current or last known
// position), nearest to (x, y) first, ties by id so the order never depends on map iteration (regenhide).
func (b *Brain) recentMechs(x, y, s float64) []*Track {
	var out []*Track
	for _, t := range b.tracks {
		if b.now-t.Seen <= s {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		di, dj := hyp(out[i].Pos[0]-x, out[i].Pos[1]-y), hyp(out[j].Pos[0]-x, out[j].Pos[1]-y)
		if di != dj {
			return di < dj
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// lastSeen (watchlast) returns the nearest mech seen within the last WatchLast s whose current or last known
// position is within 200 m of (x, y), or nil.
func (b *Brain) lastSeen(x, y float64) *Track {
	if r := b.recentMechs(x, y, b.P.WatchLast); len(r) > 0 && hyp(r[0].Pos[0]-x, r[0].Pos[1]-y) <= 200 {
		return r[0]
	}
	return nil
}

func (b *Brain) dodge(o *Obs) (dx, dy float64, laser bool) {
	me := &o.Self
	cx, cy, cz := me.Pos[0], me.Pos[1], me.Pos[2]+me.Stats.Height*0.56
	reach := me.Stats.Radius + 1.1
	for i := range o.Projectiles {
		p := &o.Projectiles[i]
		if p.Kind == "mortar" {
			continue
		}
		v2 := p.Vel[0]*p.Vel[0] + p.Vel[1]*p.Vel[1] + p.Vel[2]*p.Vel[2]
		if v2 < 1 {
			continue
		}
		tc := ((cx-p.Pos[0])*p.Vel[0] + (cy-p.Pos[1])*p.Vel[1] + (cz-p.Pos[2])*p.Vel[2]) / v2
		if tc <= 0 || tc > 0.7 {
			continue
		}
		mxx, myy := p.Pos[0]+p.Vel[0]*tc-cx, p.Pos[1]+p.Vel[1]*tc-cy
		if hyp(mxx, myy) < reach {
			vh := hyp(p.Vel[0], p.Vel[1])
			if vh < 1 {
				continue
			}
			px, py := -p.Vel[1]/vh, p.Vel[0]/vh
			if px*mxx+py*myy > 0 {
				px, py = -px, -py
			}
			dx += px
			dy += py
		}
	}
	for i := range o.Mechs {
		m := &o.Mechs[i]
		if m.Kind != "mech" || !m.Charging {
			continue
		}
		if b.P.LaserLOS > 0 && b.ffaNow {
			break // laserlos: a sidestep does not beat a laser (real data); the cover move must not be outweighed
		}
		toMe := math.Atan2(cy-m.Pos[1], cx-m.Pos[0])
		if math.Abs(angDiff(m.Aim[0], toMe)) > 0.15 {
			continue
		}
		dx += math.Cos(toMe+math.Pi/2*b.strafe) * 1.5
		dy += math.Sin(toMe+math.Pi/2*b.strafe) * 1.5
		laser = true
	}
	if l := hyp(dx, dy); l > 0.01 {
		return dx / l, dy / l, laser
	}
	return 0, 0, false
}

// laserThreats (laserlos) updates b.lzSince and returns, sorted by id, the mechs that are, or out of sight may still
// be, holding a laser charge aimed at us: seen charging with their aim within 0.15 rad of us within laser range +
// 10 m. An entry ends when the mech is seen not charging, aiming more than 0.5 rad away or beyond that range, when
// one of its laser beams is seen, or charge + hold_s + 0.3 s after we first saw this charge (it must have fired).
func (b *Brain) laserThreats(o *Obs) []*Track {
	if b.lzSince == nil {
		b.lzSince = map[int]float64{}
	}
	me := &o.Self
	x, y := me.Pos[0], me.Pos[1]
	ls := b.Cfg.Weapons["laser"]
	life := ls.Charge + ls.HoldS + 0.3
	if life < 1 {
		life = 3.05
	}
	rng := ls.Range + 10
	if ls.Range <= 0 {
		rng = 290
	}
	fired := map[int]bool{}
	for _, bm := range o.Beams {
		if bm.Kind == "laser" {
			fired[bm.By] = true
		}
	}
	for id, since := range b.lzSince {
		if b.tracks[id] == nil || fired[id] || b.now-since > life {
			delete(b.lzSince, id)
		}
	}
	var out []*Track
	for id, t := range b.tracks {
		if t.Kind != "mech" {
			continue
		}
		if t.Seen == b.now {
			off := math.Abs(angDiff(t.Aim[0], math.Atan2(y-t.Pos[1], x-t.Pos[0])))
			if !t.Charging || off > 0.5 || hyp(t.Pos[0]-x, t.Pos[1]-y) > rng || fired[id] {
				delete(b.lzSince, id)
				continue
			}
			if _, ok := b.lzSince[id]; !ok && off < 0.15 {
				b.lzSince[id] = b.now
			}
		}
		if _, ok := b.lzSince[id]; ok {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// laserCover (laserlos) runs coverMove against the laser threats and accepts it only if the hidden spot is within
// LaserLOS m (otherwise the charge lands before we get there and the move only costs position).
func (b *Brain) laserCover(o *Obs, threats []*Track, mx, my *float64) bool {
	ox, oy := *mx, *my
	if !b.coverMove(o, threats, mx, my) {
		return false
	}
	me := &o.Self
	if hyp(b.coverGoal[0]-me.Pos[0], b.coverGoal[1]-me.Pos[1]) > b.P.LaserLOS {
		*mx, *my = ox, oy
		b.coverOK = false
		return false
	}
	return true
}

// canFire (coverreload) reports whether any held weapon can fire within 0.3 s: the gun with ammo and no reload
// left beyond 0.3 s, a laser not cooling beyond 0.3 s, a mortar off cooldown, or lightning with energy for 3 shots
// and the target (if any) within its range + 6 m.
func (b *Brain) canFire(me *Self, gun, laser, light *WeaponState, target *Track) bool {
	if gun != nil && gun.Ammo > 0 && gun.Reload <= 0.3 {
		return true
	}
	if laser != nil && laser.Cooldown <= 0.3 {
		return true
	}
	if _, mortar := hasWeapon(me.Weapons, "mortar"); mortar != nil && mortar.Cooldown <= 0.3 {
		return true
	}
	if light != nil && me.Energy >= light.Stats.Energy*3 && (target == nil || target.Dist <= light.Stats.Range+6) {
		return true
	}
	return false
}

// losNow (coverreload) returns the tracks seen this tick that have a clear line to us.
// kiteAway (kiteboost): the visible mech target t is more than 15 m inside our preferred range, the move (mx, my)
// leads away from it, our energy is above KiteBoost (60 with lightning) and our laser is not charging.
func (b *Brain) kiteAway(me *Self, t *Track, mx, my float64, light bool, laser *WeaponState) bool {
	if t.Kind != "mech" || t.Seen != b.now || me.Energy <= b.P.KiteBoost || (light && me.Energy <= 60) ||
		(laser != nil && laser.Charge > 0) {
		return false
	}
	if t.Dist >= b.prefRange(me, t)-15 {
		return false
	}
	dx, dy := t.Pos[0]-me.Pos[0], t.Pos[1]-me.Pos[1]
	m, d := hyp(mx, my), hyp(dx, dy)
	return m > 0.1 && d > 0.1 && (mx*dx+my*dy)/(m*d) < -0.3
}

// cornerCover (cornercover) runs after giddy's coverMove found nothing: a wider search (up to 1000 candidate
// spots, 70 m of path) for a spot hidden from every recent mech, then from the CornerCover worst attackers only.
// A failed attempt is not repeated for 0.3 s (CPU), so in between we fight as giddy does.
func (b *Brain) cornerCover(o *Obs, seen []*Track, mx, my *float64) bool {
	if b.now-b.ccFailAt < 0.3 {
		return false
	}
	b.coverChecks, b.coverSteps = 1000, 35
	defer func() { b.coverChecks, b.coverSteps = 0, 0 }()
	if b.coverMove(o, seen, mx, my) {
		return true
	}
	if th := b.cornerThreats(seen, b.P.CornerCover); len(th) > 0 && b.coverMove(o, th, mx, my) {
		return true
	}
	b.ccFailAt = b.now
	return false
}

// cornerThreats (cornercover): the n most dangerous of the mechs that see us now (LOS this tick) or hit us in the
// last 3 s: highest recent damage rate on us first (threatRate), then nearest.
func (b *Brain) cornerThreats(seen []*Track, n int) []*Track {
	var out []*Track
	for _, t := range seen {
		if (t.Seen == b.now && t.LOS) || b.now-t.HitMe < 3 {
			out = append(out, t)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		ri, rj := b.threatRate(out[i].ID), b.threatRate(out[j].ID)
		if ri != rj {
			return ri > rj
		}
		return out[i].Dist < out[j].Dist
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}

func losNow(seen []*Track, now float64) []*Track {
	var out []*Track
	for _, t := range seen {
		if t.Seen == now && t.LOS {
			out = append(out, t)
		}
	}
	return out
}

// rescanSpin (rescan) reports whether a full spin is running or due: every 6 s gun-only, 15 s armed, only when calm
// (no damage taken for 3 s; no mech seen within 50 m; no laser holder or mech that hit us in 10 s within 150 m).
func (b *Brain) rescanSpin(o *Obs, seen []*Track, armed bool) bool {
	if b.now < b.rsUntil {
		return true
	}
	if b.now-b.lastHurt < 3 {
		return false
	}
	x, y := o.Self.Pos[0], o.Self.Pos[1]
	for _, t := range seen {
		d := hyp(t.Pos[0]-x, t.Pos[1]-y)
		if d < 50 || (d < 150 && (holds(t.Weapons, "laser") || b.now-t.HitMe < 10)) {
			return false
		}
	}
	every := 6.0
	if armed {
		every = 15
	}
	if b.now-b.rsAt < every {
		return false
	}
	tr := o.Self.Stats.TurnRate
	if tr <= 0 {
		tr = 4
	}
	b.rsAt, b.rsUntil = b.now, b.now+2*math.Pi/tr+0.1
	return true
}

func (b *Brain) reach(me *Self) float64 {
	r := 0.0
	for _, w := range me.Weapons {
		if w == nil {
			continue
		}
		switch w.Kind {
		case "gun":
			r = math.Max(r, math.Min(w.Stats.Range-10, 130))
		case "laser":
			r = math.Max(r, w.Stats.Range-5)
		case "lightning":
			r = math.Max(r, w.Stats.Range)
		}
	}
	return r
}

func (b *Brain) odds(me *Self, t *Track, now float64, opening bool) float64 {
	myW, thW := 0, len(t.Weapons)
	for _, w := range me.Weapons {
		if w != nil {
			myW++
		}
	}
	if thW == 0 {
		thW = 1
	}
	capc := func(c int) float64 { return math.Min(float64(c), 10) }
	busyMe, busyThem := 0.0, 0.0
	for _, u := range b.tracks {
		if u.ID == t.ID || now-u.Seen > 2.5 {
			continue
		}
		if now-u.HitMe < 6 {
			busyMe = 1
		}
		br := math.Atan2(u.Pos[1]-t.Pos[1], u.Pos[0]-t.Pos[0])
		if math.Abs(angDiff(t.Aim[0], br)) < 0.2 && hyp(u.Pos[0]-t.Pos[0], u.Pos[1]-t.Pos[1]) < 160 {
			busyThem = 1
		}
	}
	z := 1.5093*math.Log(math.Max(me.HP, 1)/math.Max(t.HP, 1)) + 0.7049*(capc(me.Cubes)-capc(t.Cubes))/10 +
		0.9583*float64(myW-thW) - 0.5592*(busyMe-busyThem)
	if opening {
		z += 0.0562 + 0.7475*math.Min(t.Dist, 200)/100
		toMe := math.Atan2(me.Pos[1]-t.Pos[1], me.Pos[0]-t.Pos[0])
		if math.Abs(angDiff(t.Aim[0], toMe)) > 1.75 {
			z += 0.8235
		}
	}
	return 1 / (1 + math.Exp(-z))
}

// hearShots (facesound) remembers the newest gun / laser / laser_charge / lightning sound heard near or mid
// (< 160 m) that no mech we can see this tick explains (none within 0.35 rad, about 3 sigma of the 7 degree
// bearing noise). NPC shots are a separate kind (npc_gun) and our own sounds are never reported.
func (b *Brain) hearShots(o *Obs) {
	me := &o.Self
	for _, s := range o.Sounds {
		switch s.Kind {
		case "gun", "laser", "laser_charge", "lightning":
		default:
			continue
		}
		if s.Dist != "near" && s.Dist != "mid" {
			continue
		}
		at := o.T - s.Age
		if at < b.soundAt {
			continue
		}
		explained := false
		for i := range o.Mechs {
			m := &o.Mechs[i]
			if m.Kind == "mech" && math.Abs(angDiff(math.Atan2(m.Pos[1]-me.Pos[1], m.Pos[0]-me.Pos[0]), s.Bearing)) < 0.35 {
				explained = true
				break
			}
		}
		if !explained {
			b.soundAt, b.soundBearing = at, s.Bearing
		}
	}
}

// duelOdds is the chance that we win a duel against t, from the logistic fit on 187 decisive engagements in
// recorded top-16 showdowns (vault: Data/Showdown analysis.md, Sat 22:10 GMT). For the side that shot first:
// z = 0.94 + 4.0·ln(HP_shooter/HP_target) + 0.74·[shooter holds laser] − 1.19·[target holds laser]
// + 0.68·[target's back turned]. If t has been hitting us (shot first), our chance is 1 − P(t wins as shooter).
// Used only by DuelFlee.
func (b *Brain) duelOdds(me *Self, t *Track) float64 {
	sig := func(z float64) float64 { return 1 / (1 + math.Exp(-z)) }
	_, ml := hasWeapon(me.Weapons, "laser")
	meLaser, themLaser := ml != nil, holds(t.Weapons, "laser")
	ind := func(v bool) float64 {
		if v {
			return 1
		}
		return 0
	}
	hpMe, hpThem := math.Max(me.HP, 1), math.Max(t.HP, 1)
	if ex := b.exch[t.ID]; ex != nil && b.now-ex.last < 6 && !ex.meFirst {
		// t shot first in the current exchange: its back-turned term refers to us (our aim away from t).
		toT := math.Atan2(t.Pos[1]-me.Pos[1], t.Pos[0]-me.Pos[0])
		meBack := math.Abs(angDiff(me.Aim[0], toT)) > math.Pi/2
		z := 0.94 + 4.0*math.Log(hpThem/hpMe) + 0.74*ind(themLaser) - 1.19*ind(meLaser) + 0.68*ind(meBack)
		return 1 - sig(z)
	}
	toMe := math.Atan2(me.Pos[1]-t.Pos[1], me.Pos[0]-t.Pos[0])
	themBack := math.Abs(angDiff(t.Aim[0], toMe)) > math.Pi/2
	return sig(0.94 + 4.0*math.Log(hpMe/hpThem) + 0.74*ind(meLaser) - 1.19*ind(themLaser) + 0.68*ind(themBack))
}

// armActive: the arming recipe (Arm) applies now: before ArmUntil and holding no second weapon.
func (b *Brain) armActive(me *Self) bool {
	if !b.P.Arm || b.now >= dflt(b.P.ArmUntil, 30) || (b.P.ArmStrict && !b.notPractice) ||
		(b.P.ArmWin && !b.pracOn && !b.notPractice) {
		return false
	}
	for i, w := range me.Weapons {
		if w != nil && (i > 0 || w.Kind != "gun") {
			return false
		}
	}
	return true
}

// armRunUnderFire (armfirst): with a foe, before PracAt and gun-only, keep walking to a weapon pickup within
// 90 m (chooseLoot's arm candidate) when the foe and every mech seen this tick are at least 70 m away.
func (b *Brain) armRunUnderFire(o *Obs, me *Self, foe *Track, hpFrac float64, danger bool, mx, my *float64) bool {
	if !b.P.ArmFirst || b.now >= dflt(b.base.PracAt, 20) || !b.armActive(me) || foe.Dist < 70 {
		return false
	}
	for _, t := range b.tracks {
		if t.Kind == "mech" && t.Seen == b.now && t.Dist < 70 {
			return false
		}
	}
	lc := b.chooseLoot(o, hpFrac, danger)
	if lc == nil || lc.kind != "pickup" || hyp(lc.pos[0]-me.Pos[0], lc.pos[1]-me.Pos[1]) >= 90 {
		return false
	}
	*mx, *my = b.travel(me, lc.pos[0], lc.pos[1])
	b.abAt, b.abPos = b.now+1, [2]float64{lc.pos[0], lc.pos[1]} // armboost (stored +1)
	return true
}

// losCount (armfirst) counts mechs seen in the last 5 s whose eye has line of sight to a point 1 m above pos.
func (b *Brain) losCount(pos Vec3) int {
	n := 0
	p := Vec3{pos[0], pos[1], pos[2] + 1}
	for _, t := range b.tracks {
		if t.Kind == "mech" && b.now-t.Seen < 5 && hyp(t.Pos[0]-pos[0], t.Pos[1]-pos[1]) < 230 && b.W.Clear(b.eye(t), p) {
			n++
		}
	}
	return n
}

// dflt returns v, or d when v is not set (<= 0).
func dflt(v, d float64) float64 {
	if v > 0 {
		return v
	}
	return d
}

// segDist is the distance from (px, py) to the segment (ax, ay)-(bx, by).
func segDist(px, py, ax, ay, bx, by float64) float64 {
	dx, dy := bx-ax, by-ay
	l2 := dx*dx + dy*dy
	t := 0.0
	if l2 > 1e-9 {
		t = clamp(((px-ax)*dx+(py-ay)*dy)/l2, 0, 1)
	}
	return hyp(px-(ax+t*dx), py-(ay+t*dy))
}

// tpOpp (tpjoin) returns the mech t is fighting and its distance to t: the nearest other mech seen this tick
// within 230 m of t that traded a beam with t this tick, or that t aims at / that aims at t (0.25 rad), where
// the aim counts only if it is mutual or t lost HP to someone else in the last 2 s. Never us (not a track).
func (b *Brain) tpOpp(o *Obs, t *Track) (*Track, float64) {
	var opp *Track
	od := 230.0
	for _, u := range b.tracks {
		if u == t || u.Seen != b.now {
			continue
		}
		d := hyp(u.Pos[0]-t.Pos[0], u.Pos[1]-t.Pos[1])
		if d >= od {
			continue
		}
		tAims := math.Abs(angDiff(t.Aim[0], math.Atan2(u.Pos[1]-t.Pos[1], u.Pos[0]-t.Pos[0]))) < 0.25
		uAims := math.Abs(angDiff(u.Aim[0], math.Atan2(t.Pos[1]-u.Pos[1], t.Pos[0]-u.Pos[0]))) < 0.25
		beam := false
		for _, bm := range o.Beams {
			for k := 1; k < len(bm.Path); k++ {
				p := bm.Path[k]
				if (bm.By == u.ID && hyp(p[0]-t.Pos[0], p[1]-t.Pos[1]) < 4) || (bm.By == t.ID && hyp(p[0]-u.Pos[0], p[1]-u.Pos[1]) < 4) {
					beam = true
				}
			}
		}
		if beam || (tAims && uAims) || ((tAims || uAims) && b.now-t.HurtAt < 2) {
			opp, od = u, d
		}
	}
	return opp, od
}

// lrHP (lrlaser) is the HP ratio we need over a mech before opening on it at long range (LRHP, 0 = 1.0).
func (b *Brain) lrHP() float64 {
	if b.P.LRHP > 0 {
		return b.P.LRHP
	}
	return 1
}

// lrOpen (lrlaser) reports whether to open on t at long range with the laser (see LRLaser). aimed and off are
// giddy's test of t's aim at us (within 0.22 rad) and that aim error in rad.
func (b *Brain) lrOpen(o *Obs, me *Self, t *Track, laser *WeaponState, aimed bool, off, maxT float64) bool {
	if laser == nil || laser.Cooldown > 0.3 || t.Seen != b.now || !t.LOS || b.retreat || b.now < b.oddsFleeUntil {
		return false
	}
	if t.Dist < b.P.EngageDist || t.Dist > math.Min(b.P.LRLaser, maxT) || me.HP < b.lrHP()*t.HP {
		return false
	}
	if t.Charging && off < 0.5 {
		return false // charging a laser at us (a wider cone than "aimed": it can swing that far before release)
	}
	others := 0
	for _, u := range b.tracks {
		if u != t && u.Seen == b.now && u.Dist < 150 {
			others++
		}
	}
	if others > 1 {
		return false // crowd: keep out of three-way fights
	}
	if aimed {
		// It has noticed us: open only while it is busy with another mech.
		if opp, _ := b.tpOpp(o, t); opp == nil {
			return false
		}
	}
	return true
}

// sqLROpen (sqlrlaser) is lrOpen for squad mode: range SqLRLaser, HP >= its HP, "not noticed" = aim > 1.0 rad off us.
func (b *Brain) sqLROpen(o *Obs, me *Self, t *Track, laser *WeaponState, off, maxT float64) bool {
	if laser == nil || laser.Cooldown > 0.3 || t.Seen != b.now || !t.LOS || b.retreat || b.now < b.oddsFleeUntil {
		return false
	}
	if t.Dist < b.P.EngageDist || t.Dist > math.Min(b.P.SqLRLaser, maxT) || me.HP < t.HP {
		return false
	}
	if t.Charging && off < 0.5 {
		return false
	}
	others := 0
	for _, u := range b.tracks {
		if u != t && u.Seen == b.now && u.Dist < 150 {
			others++
		}
	}
	if others > 1 {
		return false
	}
	if off <= 1.0 {
		if opp, _ := b.tpOpp(o, t); opp == nil {
			return false
		}
	}
	return true
}

func squadParams(p Params) Params {
	p.Ghost = false
	p.Peek = false
	p.LaserFirst = true
	p.LaserRange = 120
	p.LootRadius = 150
	p.CrateUntil = 1e9
	p.Stick = 120
	p.HPw = 0.2
	p.LowBonus = 100
	p.MaxTarget = 240
	p.LoseRule = true
	p.FightLoot = 70
	p.FarmHP = 0.5

	p.LaserSeek = 240
	p.LaserUp = true
	p.Lethal = true
	p.Finish = 20
	p.Stick = 200
	p.LowBonus = 200
	p.LethalK = 0.7
	p.HPw = 0.15
	p.HPw = 0.18
	if p.SqHPw > 0 {
		p.HPw = p.SqHPw
	}
	if p.SqEngage > 0 {
		p.EngageDist = p.SqEngage // sqengage: squad-mode engage radius only
	}
	if p.SqUseDrops {
		p.UseDrops = true
	}

	if p.SqLaserRange > 0 {
		p.LaserRange = p.SqLaserRange
	}
	if p.SqFinish > 0 {
		p.Finish = p.SqFinish
	}
	// Keep practice exactly as giddy tuned it: our FFA changes stay off.
	p.FightBoost, p.DodgeBoost, p.ThreatBoost, p.SpeedUp = 0, 0, 0, false
	p.TPJoin, p.BackOpen, p.PreCharge, p.PeekCD = 0, 0, 0, 0
	p.LRLaser, p.LRHP, p.LRKeep = 0, 0, 0
	p.RegenHide, p.WatchLast, p.HomeLOS, p.ChargeEngage = 0, 0, 0, 0
	p.FFADrops, p.CubeStop, p.EarlyRetreat = 0, false, 0
	p.LaserLOS = 0
	p.LateR = 70
	p.HoldOut = 0
	p.LateAlive = 3
	p.Arm = p.Arm && (p.ArmPrac || p.ArmWin)
	p.CubePush = 0

	return p
}

func (b *Brain) cubeCap() int {
	c := b.Cfg.Cubes.StatCap
	if g := int(b.P.CubeGoal); g > c {
		c = g
	}
	return c
}

func (b *Brain) rearMult() float64 {
	if m := b.Cfg.Mech.RearMult; m > 1 {
		return m
	}
	return 1
}

func (b *Brain) showsBack(t *Track, x, y float64) bool {
	arc := b.Cfg.Mech.FrontArcDeg
	if arc <= 0 {
		arc = 180
	}
	return math.Abs(angDiff(t.Aim[0], math.Atan2(y-t.Pos[1], x-t.Pos[0]))) > arc*math.Pi/360+0.1
}

// backHold (backopen) reports whether to hold our opening fire on t, and whether t may see us (inside its view
// wedge + 0.1 rad and range). A fresh opener is held until t's aim is more than 90° + 0.15 rad off us, for at
// most BackOpen s per target (timer per mech id, restarted after BackOpen + 8 s). A held laser charge is let go
// as soon as t may see us, before it can dodge or answer.
func (b *Brain) backHold(me *Self, t *Track, hpFrac float64, charging bool) (hold, sees bool) {
	now := b.now
	near, fov, vr := b.Cfg.Vision.NearRange, b.Cfg.Vision.FovDeg, b.Cfg.Vision.Range
	if near <= 0 {
		near = 45
	}
	if fov <= 0 {
		fov = 120
	}
	if vr <= 0 {
		vr = 200
	}
	if now-t.HitMe < 6 || t.Charging || hpFrac <= 0.6 || t.Dist <= near+2 {
		return false, false
	}
	if at, ok := b.myHitAt[t.ID]; ok && now-at < 8 {
		return false, false
	}
	off := math.Abs(angDiff(t.Aim[0], math.Atan2(me.Pos[1]-t.Pos[1], me.Pos[0]-t.Pos[0])))
	sees = off < fov*math.Pi/360+0.1 && t.Dist < vr+10
	if off > math.Pi/2+0.15 || sees {
		// Back turned: fire now. Target can see us: holding would hand it the first shot, so fire too.
		return false, sees
	}
	if b.boWait == nil {
		b.boWait = map[int]float64{}
	}
	ws, ok := b.boWait[t.ID]
	if !ok || now-ws > b.P.BackOpen+8 {
		ws = now
		b.boWait[t.ID] = ws
	}
	return now-ws < b.P.BackOpen, sees
}

// holdingOut reports whether a ghost should stay away from the final circle's centre for now.
func (b *Brain) holdingOut(z *Zone, alive int) bool {
	return b.P.Ghost && b.P.HoldOut > 0 && b.now < b.P.HoldOut && z.NextRadius <= 30 && alive > 2
}

// holdHome picks a hidden spot inside the current circle, between HoldMin and HoldMin+25 m from the final
// centre. The normal zone deadline logic (zoneMust) still pulls the bot in before the circle reaches it.
// force (homelos) re-chooses at once.
func (b *Brain) holdHome(o *Obs, force bool) (float64, float64, bool) {
	z := &o.Zone
	c := b.curZone(z)
	minD := b.P.HoldMin
	if minD <= 0 {
		minD = 40
	}
	valid := func(px, py float64) bool {
		d := hyp(px-z.NextCenter[0], py-z.NextCenter[1])
		return d >= minD*0.8 && hyp(px-c.X, py-c.Y) <= c.R-12
	}
	if !b.homeSet || !valid(b.homeXY[0], b.homeXY[1]) || b.now-b.homeAt > 12 || force {
		b.homeAt = b.now
		best, bestC := [2]float64{}, math.Inf(1)
		for k := 0; k < 48; k++ {
			a := b.rng.Float64() * 2 * math.Pi
			r := minD + b.rng.Float64()*25
			px, py := z.NextCenter[0]+math.Cos(a)*r, z.NextCenter[1]+math.Sin(a)*r
			if !valid(px, py) || !b.W.InBounds(px, py) || !b.W.Reachable(px, py) {
				continue
			}
			if cst := b.homeCost(o, px, py) + 0.5*r; cst < bestC {
				best, bestC = [2]float64{px, py}, cst
			}
		}
		if math.IsInf(bestC, 1) {
			// No valid spot (circle too small): fall back to giddy's behaviour.
			return z.NextCenter[0], z.NextCenter[1], true
		}
		if !b.homeSet || !valid(b.homeXY[0], b.homeXY[1]) || bestC < b.homeCost(o, b.homeXY[0], b.homeXY[1])+0.5*hyp(b.homeXY[0]-z.NextCenter[0], b.homeXY[1]-z.NextCenter[1])-15 {
			b.homeXY, b.homeSet = best, true
		}
	}
	return b.homeXY[0], b.homeXY[1], b.homeSet
}

// applyFlags overrides parameters from a "key=value,key=value" string (env MECH_FLAGS). It is only used
// by local experiments; with the variable unset the bot runs DefaultParams unchanged. Booleans take 0/1.
func applyFlags(p *Params, spec string) {
	for _, kv := range strings.Split(spec, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(kv), "=")
		if !ok {
			continue
		}
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			continue
		}
		on := f != 0
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "later":
			p.LateR = f
		case "holdout":
			p.HoldOut = f
		case "holdmin":
			p.HoldMin = f
		case "lethal":
			p.Lethal = on
		case "nanoup":
			p.NanoUp = on
		case "laserup":
			p.LaserUp = on
		case "vulture":
			p.Vulture = f
		case "ambush":
			p.Ambush = f
		case "odds":
			p.Odds = f
		case "oddsflee":
			p.OddsFlee = f
		case "engage":
			p.EngageDist = f
		case "retreat":
			p.RetreatHP = f
		case "recover":
			p.RecoverHP = f
		case "crateuntil":
			p.CrateUntil = f
		case "lootradius":
			p.LootRadius = f
		case "usedrops":
			p.UseDrops = on
		case "practice":
			p.Practice = on
		case "laserfirst":
			p.LaserFirst = on
		case "ffamin":
			p.FFAMin = f
		case "sqlaser":
			p.SqLaserRange = f
		case "sqfinish":
			p.SqFinish = f
		case "pracat":
			p.PracAt = f
		case "beamffa":
			p.BeamFFA = on
		case "laserseek":
			p.LaserSeek = f
		case "lethalk":
			p.LethalK = f
		case "backstab":
			p.Backstab = f
		case "speedup":
			p.SpeedUp = on
		case "fightboost":
			p.FightBoost = f
		case "dodgeboost":
			p.DodgeBoost = f
		case "sqspeed":
			p.SqSpeed = on
		case "sqdodge":
			p.SqDodge = f
		case "sqhunt":
			p.SqHunt = f
		case "sqengage":
			p.SqEngage = f
		case "regenlock":
			p.RegenLock = on
		case "regenlockhp":
			p.RegenLockHP = f
		case "threatboost":
			p.ThreatBoost = f
		case "opening":
			p.Opening = on
		case "openuntil":
			p.OpenUntil = f
		case "gunonlyout":
			p.GunOnlyOut = on
		case "lateal":
			p.LateAlive = int(f)
		case "lightlos":
			p.LightLOS = on
		case "duelflee":
			p.DuelFlee = f
		case "facesound":
			p.FaceSound = on
		case "hunthide":
			p.HuntHide = on
		case "gunonlyr":
			p.GunOnlyR = f
		case "crowdr":
			p.CrowdR = f
		case "homefrac":
			p.HomeFrac = f
		case "cfgnosquad":
			p.CfgNoSquad = on
		case "cfgffa":
			p.CfgFFA = on
		case "cfggate":
			p.CfgGate = on
		case "squsedrops":
			p.SqUseDrops = on
		case "sqhpw":
			p.SqHPw = f
		case "ffastrictn":
			p.FFAStrictN = f
		case "ffastrict":
			p.FFAStrict = on
		case "keeplaser":
			p.KeepLaser = on
		case "arm":
			p.Arm = on
		case "armfirst":
			p.ArmFirst = on
		case "armprac":
			p.ArmPrac = on
		case "armstrict":
			p.ArmStrict = on
		case "armwin":
			p.ArmWin = on
		case "armuntil":
			p.ArmUntil = f
		case "armr":
			p.ArmR = f
		case "armrfar":
			p.ArmRFar = f
		case "armv":
			p.ArmV = f
		case "armroute":
			p.ArmRoute = f
		case "armfire":
			p.ArmFire = f
		case "armcrate":
			p.ArmCrate = f
		case "scan":
			p.Scan = f
		case "cubepush":
			p.CubePush = f
		case "cubepushk":
			p.CubePushK = f
		case "tpjoin":
			p.TPJoin = f
		case "tphp":
			p.TPHP = f
		case "backopen":
			p.BackOpen = f
		case "precharge":
			p.PreCharge = f
		case "peekcd":
			p.PeekCD = int(f)
		case "lrlaser":
			p.LRLaser = f
		case "lrhp":
			p.LRHP = f
		case "lrkeep":
			p.LRKeep = f
		case "regenhide":
			p.RegenHide = f
		case "watchlast":
			p.WatchLast = f
		case "homelos":
			p.HomeLOS = f
		case "rescan":
			p.Rescan = on
		case "sqbackopen":
			p.SqBackOpen = on
		case "sqlrlaser":
			p.SqLRLaser = f
		case "armboost":
			p.ArmBoost = on
		case "sqjink":
			p.SqJink = on
		case "chargeengage":
			p.ChargeEngage = f
		case "earlyspace":
			p.EarlySpace = f
		case "earlyspacer":
			p.EarlySpaceR = f
		case "earlyspacekeep":
			p.EarlySpaceKeep = f
		case "laserlos":
			p.LaserLOS = f
		case "rearavoid":
			p.RearAvoid = f
		case "coverreload":
			p.CoverReload = on
		case "ffadrops":
			p.FFADrops = f
		case "cubestop":
			p.CubeStop = on
		case "dropalive":
			p.DropAlive = int(f)
		case "earlyretreat":
			p.EarlyRetreat = f
		case "lcdpunish":
			p.LCDPunish = f
		case "lightpad":
			p.LightPad = f
		case "smartfire":
			p.SmartFire = f
		case "threatfocus":
			p.ThreatFocus = f
		case "cornercover":
			p.CornerCover = int(f)
		case "cautioust":
			p.CautiousT = f
		case "pracrim":
			p.PracRim = f
		case "squadrim":
			if on {
				p.SqRim, p.SqLootIn, p.SqBand, p.SqRet = 0.9, 50, true, 0.30
			} else {
				p.SqRim, p.SqLootIn, p.SqBand, p.SqRet = 0, 0, false, 0
			}
		case "sqrim":
			p.SqRim = f
		case "sqloot":
			p.SqLootIn = f
		case "sqband":
			p.SqBand = on
		case "sqret":
			p.SqRet = f
		case "sqretup":
			p.SqRetUp = f
		case "pengage":
			p.PEngage = f
		case "sqcorner":
			p.SqCorner = on
		case "kiteboost":
			p.KiteBoost = f
		case "kitew":
			p.KiteW = f
		case "pracregen":
			p.PracRegen = f
		}
	}
}

// beamCheck latches notPractice when a beam from one mech lands on a different mech that is not us and that
// lost HP since we last saw it (and not from our own hits). It runs before the tracks are updated, so a track
// still holds the HP from our previous sighting.
func (b *Brain) beamCheck(o *Obs) {
	me := &o.Self
	if b.myHitAt == nil {
		b.myHitAt = map[int]float64{}
	}
	if b.exch == nil {
		b.exch = map[int]*exchange{}
	}
	for _, e := range o.Events {
		if e.Type == "hit" {
			b.myHitAt[e.Target] = o.T
			b.noteExchange(e.Target, true, o.T)
		}
		if e.Type == "damaged" && e.By != nil {
			b.noteExchange(*e.By, false, o.T)
		}
	}
	if !b.P.BeamFFA || b.notPractice || len(o.Beams) == 0 {
		return
	}
	myR := b.Cfg.Mech.Radius*b.scaleOf(me.Cubes) + 2
	for _, bm := range o.Beams {
		if bm.By == me.ID || len(bm.Path) < 2 {
			continue
		}
		end := bm.Path[1]
		if bm.Kind == "laser" {
			end = bm.Path[len(bm.Path)-1]
		}
		if hyp(end[0]-me.Pos[0], end[1]-me.Pos[1]) < myR {
			continue // aimed at us
		}
		if st := bm.Path[0]; math.Sqrt((end[0]-st[0])*(end[0]-st[0])+(end[1]-st[1])*(end[1]-st[1])+(end[2]-st[2])*(end[2]-st[2])) < 6 {
			continue // stopped next to the shooter (a wall), not a fight
		}
		for i := range o.Mechs {
			m := &o.Mechs[i]
			if m.Kind != "mech" || m.ID == bm.By || m.ID == me.ID {
				continue
			}
			sc := b.scaleOf(m.Cubes)
			if hyp(end[0]-m.Pos[0], end[1]-m.Pos[1]) > b.Cfg.Mech.Radius*sc+1.5 ||
				end[2] < m.Pos[2]-1.5 || end[2] > m.Pos[2]+b.Cfg.Mech.Height*sc+1.5 {
				continue
			}
			t := b.tracks[m.ID]
			if t == nil || o.T-t.Seen > 1.0 || m.HP >= t.HP-1 {
				continue // must have lost HP since our last sighting
			}
			if at, ok := b.myHitAt[m.ID]; ok && o.T-at < 1.0 {
				continue // we hit it ourselves
			}
			b.notPractice = true
			return
		}
	}
}

// isShowdownCfg reports whether the match config has the schedule seen in every recorded top-16 showdown and in
// the kit's recorded hello: zone starts at 560 m, first phase holds 60 s and shrinks 50 s, drops at 70/140/210 s.
func isShowdownCfg(c *Config) bool {
	eq := func(a, b float64) bool { return math.Abs(a-b) < 1e-6 }
	z := c.Zone
	if !eq(z.StartRadius, 560) || len(z.Phases) == 0 || !eq(z.Phases[0].HoldS, 60) || !eq(z.Phases[0].ShrinkS, 50) {
		return false
	}
	t := c.Drops.Times
	return len(t) == 3 && eq(t[0], 70) && eq(t[1], 140) && eq(t[2], 210)
}

// exchange records who opened the current exchange of fire with one mech (an exchange ends after 6 s without
// hits either way). duelOdds uses it to pick the first-shooter side of the duel model.
type exchange struct {
	last    float64
	meFirst bool
}

func (b *Brain) noteExchange(id int, byMe bool, now float64) {
	ex := b.exch[id]
	if ex == nil {
		ex = &exchange{last: -100}
		b.exch[id] = ex
	}
	if now-ex.last > 6 {
		ex.meFirst = byMe
	}
	ex.last = now
}

// safeFallback is the crash breaker: after 3 panics in a match it switches every one of our FFA additions off and
// plays plain giddy for the rest of the match, so a bug in FFA-only code cannot leave us standing still.
func (b *Brain) safeFallback() {
	b.safeMode = true
	off := func(p *Params) {
		p.DuelFlee, p.TPJoin, p.BackOpen, p.PreCharge, p.PeekCD = 0, 0, 0, 0, 0
		p.LightLOS, p.FaceSound, p.GunOnlyOut, p.KeepLaser, p.HuntHide, p.BeamFFA = false, false, false, false, false, false
		p.CrowdR, p.HomeFrac, p.GunOnlyR, p.LateAlive = 0, 0, 0, 3
		p.FightBoost, p.DodgeBoost, p.ThreatBoost, p.SpeedUp, p.Vulture = 0, 0, 0, false, 0
		p.LateR, p.HoldOut = 70, 0
		p.LRLaser, p.LRHP, p.LRKeep = 0, 0, 0
		p.RegenHide, p.WatchLast, p.HomeLOS, p.ChargeEngage = 0, 0, 0, 0
		p.SqBackOpen, p.SqLRLaser = false, 0
		p.EarlySpace, p.EarlySpaceR, p.EarlySpaceKeep = 0, 0, 0
		p.Rescan = false
		p.RearAvoid, p.CoverReload = 0, false
		p.FFADrops, p.CubeStop, p.EarlyRetreat = 0, false, 0
		p.LaserLOS = 0
		p.LCDPunish, p.LightPad, p.SmartFire = 0, 0, 0
		p.ThreatFocus, p.CornerCover, p.KiteBoost, p.KiteW, p.PracRegen = 0, 0, 0, 0, 0
		p.CautiousT, p.PracRim = 0, 0
		p.Arm, p.Scan, p.CubePush, p.ArmFirst = false, 0, 0, false
		p.SqRim, p.SqLootIn, p.SqBand, p.SqRet = 0, 0, false, 0
		p.SqRetUp, p.SqCorner = 0, false
		p.PEngage = 0
		p.SqSpeed, p.SqDodge, p.SqEngage = false, 0, 0
		p.SqHunt = 0
		p.RegenLock, p.RegenLockHP = false, 0
		p.ArmBoost, p.SqJink = false, false
	}
	off(&b.base)
	b.regenOn = false
	b.sqRetOn = false
	if b.pracOn {
		b.P = squadParams(b.base)
	} else {
		b.P = b.base
	}
}
