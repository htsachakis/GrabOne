export namespace config {
	
	export class Preferences {
	    downloadType: string;
	    container: string;
	    embedMetadata: boolean;
	    embedChapters: boolean;
	    embedThumbnail: boolean;
	    saveThumbnail: boolean;
	    saveDescription: boolean;
	    saveJson: boolean;
	    downloadSubtitles: boolean;
	    embedSubtitles: boolean;
	    keepSubtitles: boolean;
	    subtitleFormat: string;
	    subtitleLanguages: string[];
	    audioConversionFormat: string;
	
	    static createFrom(source: any = {}) {
	        return new Preferences(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.downloadType = source["downloadType"];
	        this.container = source["container"];
	        this.embedMetadata = source["embedMetadata"];
	        this.embedChapters = source["embedChapters"];
	        this.embedThumbnail = source["embedThumbnail"];
	        this.saveThumbnail = source["saveThumbnail"];
	        this.saveDescription = source["saveDescription"];
	        this.saveJson = source["saveJson"];
	        this.downloadSubtitles = source["downloadSubtitles"];
	        this.embedSubtitles = source["embedSubtitles"];
	        this.keepSubtitles = source["keepSubtitles"];
	        this.subtitleFormat = source["subtitleFormat"];
	        this.subtitleLanguages = source["subtitleLanguages"];
	        this.audioConversionFormat = source["audioConversionFormat"];
	    }
	}
	export class Config {
	    ytDlpPath: string;
	    ffmpegPath: string;
	    ffprobePath: string;
	    outputDirectory: string;
	    filenameTemplate: string;
	    theme: string;
	    maxConcurrentDownloads: number;
	    autoCheckUpdates: boolean;
	    skippedUpdateVersion: string;
	    cookieSource: string;
	    cookieBrowser: string;
	    cookieFile: string;
	    preferences: Preferences;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ytDlpPath = source["ytDlpPath"];
	        this.ffmpegPath = source["ffmpegPath"];
	        this.ffprobePath = source["ffprobePath"];
	        this.outputDirectory = source["outputDirectory"];
	        this.filenameTemplate = source["filenameTemplate"];
	        this.theme = source["theme"];
	        this.maxConcurrentDownloads = source["maxConcurrentDownloads"];
	        this.autoCheckUpdates = source["autoCheckUpdates"];
	        this.skippedUpdateVersion = source["skippedUpdateVersion"];
	        this.cookieSource = source["cookieSource"];
	        this.cookieBrowser = source["cookieBrowser"];
	        this.cookieFile = source["cookieFile"];
	        this.preferences = this.convertValues(source["preferences"], Preferences);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace dependencies {
	
	export class DependencyStatus {
	    name: string;
	    displayName: string;
	    available: boolean;
	    path: string;
	    version: string;
	    source: string;
	    error?: string;
	    required: boolean;
	    managed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DependencyStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.displayName = source["displayName"];
	        this.available = source["available"];
	        this.path = source["path"];
	        this.version = source["version"];
	        this.source = source["source"];
	        this.error = source["error"];
	        this.required = source["required"];
	        this.managed = source["managed"];
	    }
	}
	export class InstallStep {
	    title: string;
	    command?: string;
	    url?: string;
	    detail: string;
	
	    static createFrom(source: any = {}) {
	        return new InstallStep(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.command = source["command"];
	        this.url = source["url"];
	        this.detail = source["detail"];
	    }
	}
	export class Guidance {
	    name: string;
	    displayName: string;
	    summary: string;
	    steps: InstallStep[];
	    afterInstall: string;
	
	    static createFrom(source: any = {}) {
	        return new Guidance(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.displayName = source["displayName"];
	        this.summary = source["summary"];
	        this.steps = this.convertValues(source["steps"], InstallStep);
	        this.afterInstall = source["afterInstall"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class Set {
	    ytDlp: DependencyStatus;
	    ffmpeg: DependencyStatus;
	    ffprobe: DependencyStatus;
	
	    static createFrom(source: any = {}) {
	        return new Set(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ytDlp = this.convertValues(source["ytDlp"], DependencyStatus);
	        this.ffmpeg = this.convertValues(source["ffmpeg"], DependencyStatus);
	        this.ffprobe = this.convertValues(source["ffprobe"], DependencyStatus);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace downloads {
	
	export class Metadata {
	    title: string;
	    uploader: string;
	    platform: string;
	    platformSlug: string;
	    thumbnailUrl: string;
	    contentType: string;
	    durationText: string;
	
	    static createFrom(source: any = {}) {
	        return new Metadata(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.uploader = source["uploader"];
	        this.platform = source["platform"];
	        this.platformSlug = source["platformSlug"];
	        this.thumbnailUrl = source["thumbnailUrl"];
	        this.contentType = source["contentType"];
	        this.durationText = source["durationText"];
	    }
	}
	export class Progress {
	    downloadId: string;
	    status: string;
	    stage: string;
	    completedStages: string[];
	    percent: number;
	    overallPercent: number;
	    downloadedBytes: number;
	    totalBytes: number;
	    totalIsEstimate: boolean;
	    speedBytesPerSecond: number;
	    etaSeconds: number;
	    filename: string;
	    streamLabel: string;
	    itemIndex: number;
	    itemCount: number;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new Progress(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.downloadId = source["downloadId"];
	        this.status = source["status"];
	        this.stage = source["stage"];
	        this.completedStages = source["completedStages"];
	        this.percent = source["percent"];
	        this.overallPercent = source["overallPercent"];
	        this.downloadedBytes = source["downloadedBytes"];
	        this.totalBytes = source["totalBytes"];
	        this.totalIsEstimate = source["totalIsEstimate"];
	        this.speedBytesPerSecond = source["speedBytesPerSecond"];
	        this.etaSeconds = source["etaSeconds"];
	        this.filename = source["filename"];
	        this.streamLabel = source["streamLabel"];
	        this.itemIndex = source["itemIndex"];
	        this.itemCount = source["itemCount"];
	        this.message = source["message"];
	    }
	}
	export class View {
	    id: string;
	    url: string;
	    metadata: Metadata;
	    status: string;
	    stage: string;
	    completedStages: string[];
	    progress: Progress;
	    downloadType: string;
	    outputDirectory: string;
	    finalPath: string;
	    command: string;
	    error?: ytdlp.Error;
	    fileSummary?: ffmpeg.FileSummary;
	    createdAt: string;
	    startedAt?: string;
	    finishedAt?: string;
	    queuePosition: number;
	
	    static createFrom(source: any = {}) {
	        return new View(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.url = source["url"];
	        this.metadata = this.convertValues(source["metadata"], Metadata);
	        this.status = source["status"];
	        this.stage = source["stage"];
	        this.completedStages = source["completedStages"];
	        this.progress = this.convertValues(source["progress"], Progress);
	        this.downloadType = source["downloadType"];
	        this.outputDirectory = source["outputDirectory"];
	        this.finalPath = source["finalPath"];
	        this.command = source["command"];
	        this.error = this.convertValues(source["error"], ytdlp.Error);
	        this.fileSummary = this.convertValues(source["fileSummary"], ffmpeg.FileSummary);
	        this.createdAt = source["createdAt"];
	        this.startedAt = source["startedAt"];
	        this.finishedAt = source["finishedAt"];
	        this.queuePosition = source["queuePosition"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace ffmpeg {
	
	export class FileSummary {
	    path: string;
	    container: string;
	    duration: number;
	    sizeBytes: number;
	    videoCodec: string;
	    audioCodec: string;
	    width: number;
	    height: number;
	    subtitleTracks: number;
	    chapterCount: number;
	
	    static createFrom(source: any = {}) {
	        return new FileSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.container = source["container"];
	        this.duration = source["duration"];
	        this.sizeBytes = source["sizeBytes"];
	        this.videoCodec = source["videoCodec"];
	        this.audioCodec = source["audioCodec"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.subtitleTracks = source["subtitleTracks"];
	        this.chapterCount = source["chapterCount"];
	    }
	}

}

export namespace ghrelease {
	
	export class Asset {
	    name: string;
	    url: string;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new Asset(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.url = source["url"];
	        this.size = source["size"];
	    }
	}

}

export namespace main {
	
	export class AnalyzeRequest {
	    url: string;
	    collectionMode: string;
	
	    static createFrom(source: any = {}) {
	        return new AnalyzeRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.collectionMode = source["collectionMode"];
	    }
	}
	export class SelectionDefaults {
	    downloadType: string;
	    combinedFormatId: string;
	    videoFormatId: string;
	    audioFormatId: string;
	    container: string;
	    resolvedContainer: string;
	    subtitleLanguages: string[];
	
	    static createFrom(source: any = {}) {
	        return new SelectionDefaults(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.downloadType = source["downloadType"];
	        this.combinedFormatId = source["combinedFormatId"];
	        this.videoFormatId = source["videoFormatId"];
	        this.audioFormatId = source["audioFormatId"];
	        this.container = source["container"];
	        this.resolvedContainer = source["resolvedContainer"];
	        this.subtitleLanguages = source["subtitleLanguages"];
	    }
	}
	export class AnalyzeResponse {
	    success: boolean;
	    media?: media.MediaInfo;
	    error?: ytdlp.Error;
	    resolutionPresets: media.ResolutionPreset[];
	    defaults: SelectionDefaults;
	    canMerge: boolean;
	    canConvertAudio: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AnalyzeResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.media = this.convertValues(source["media"], media.MediaInfo);
	        this.error = this.convertValues(source["error"], ytdlp.Error);
	        this.resolutionPresets = this.convertValues(source["resolutionPresets"], media.ResolutionPreset);
	        this.defaults = this.convertValues(source["defaults"], SelectionDefaults);
	        this.canMerge = source["canMerge"];
	        this.canConvertAudio = source["canConvertAudio"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AppInfo {
	    name: string;
	    version: string;
	    versionLabel: string;
	    description: string;
	    legalNotice: string;
	    applicationDir: string;
	    settingsPath: string;
	    logDirectory: string;
	
	    static createFrom(source: any = {}) {
	        return new AppInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	        this.versionLabel = source["versionLabel"];
	        this.description = source["description"];
	        this.legalNotice = source["legalNotice"];
	        this.applicationDir = source["applicationDir"];
	        this.settingsPath = source["settingsPath"];
	        this.logDirectory = source["logDirectory"];
	    }
	}
	export class DownloadRequest {
	    url: string;
	    title: string;
	    downloadType: string;
	    videoFormatId: string;
	    audioFormatId: string;
	    combinedFormatId: string;
	    maxHeight: number;
	    container: string;
	    embedMetadata: boolean;
	    embedChapters: boolean;
	    embedThumbnail: boolean;
	    saveThumbnail: boolean;
	    saveDescription: boolean;
	    saveJson: boolean;
	    downloadSubtitles: boolean;
	    embedSubtitles: boolean;
	    keepSubtitles: boolean;
	    subtitleLanguages: string[];
	    subtitleFormat: string;
	    audioConversionFormat: string;
	    outputDirectory: string;
	    collectionMode: string;
	    playlistItems: string;
	
	    static createFrom(source: any = {}) {
	        return new DownloadRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.title = source["title"];
	        this.downloadType = source["downloadType"];
	        this.videoFormatId = source["videoFormatId"];
	        this.audioFormatId = source["audioFormatId"];
	        this.combinedFormatId = source["combinedFormatId"];
	        this.maxHeight = source["maxHeight"];
	        this.container = source["container"];
	        this.embedMetadata = source["embedMetadata"];
	        this.embedChapters = source["embedChapters"];
	        this.embedThumbnail = source["embedThumbnail"];
	        this.saveThumbnail = source["saveThumbnail"];
	        this.saveDescription = source["saveDescription"];
	        this.saveJson = source["saveJson"];
	        this.downloadSubtitles = source["downloadSubtitles"];
	        this.embedSubtitles = source["embedSubtitles"];
	        this.keepSubtitles = source["keepSubtitles"];
	        this.subtitleLanguages = source["subtitleLanguages"];
	        this.subtitleFormat = source["subtitleFormat"];
	        this.audioConversionFormat = source["audioConversionFormat"];
	        this.outputDirectory = source["outputDirectory"];
	        this.collectionMode = source["collectionMode"];
	        this.playlistItems = source["playlistItems"];
	    }
	}
	export class PreviewResponse {
	    success: boolean;
	    command: string;
	    args: string[];
	    error?: ytdlp.Error;
	    resolvedContainer: string;
	
	    static createFrom(source: any = {}) {
	        return new PreviewResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.command = source["command"];
	        this.args = source["args"];
	        this.error = this.convertValues(source["error"], ytdlp.Error);
	        this.resolvedContainer = source["resolvedContainer"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class SettingsResponse {
	    settings: config.Config;
	    dependencies: dependencies.Set;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new SettingsResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.settings = this.convertValues(source["settings"], config.Config);
	        this.dependencies = this.convertValues(source["dependencies"], dependencies.Set);
	        this.error = source["error"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class StartResponse {
	    success: boolean;
	    download?: downloads.View;
	    error?: ytdlp.Error;
	
	    static createFrom(source: any = {}) {
	        return new StartResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.download = this.convertValues(source["download"], downloads.View);
	        this.error = this.convertValues(source["error"], ytdlp.Error);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ToolInstallResponse {
	    success: boolean;
	    tool: string;
	    version: string;
	    directory: string;
	    dependencies: dependencies.Set;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ToolInstallResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.tool = source["tool"];
	        this.version = source["version"];
	        this.directory = source["directory"];
	        this.dependencies = this.convertValues(source["dependencies"], dependencies.Set);
	        this.error = source["error"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ToolSource {
	    name: string;
	    displayName: string;
	    projectUrl: string;
	    licence: string;
	    provides: string[];
	
	    static createFrom(source: any = {}) {
	        return new ToolSource(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.displayName = source["displayName"];
	        this.projectUrl = source["projectUrl"];
	        this.licence = source["licence"];
	        this.provides = source["provides"];
	    }
	}
	export class UpdateStatus {
	    checked: boolean;
	    available: boolean;
	    currentVersion: string;
	    latestVersion?: string;
	    release?: updater.Release;
	    downloading: boolean;
	    downloadedPath?: string;
	    skipped: boolean;
	    checkedAt?: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.checked = source["checked"];
	        this.available = source["available"];
	        this.currentVersion = source["currentVersion"];
	        this.latestVersion = source["latestVersion"];
	        this.release = this.convertValues(source["release"], updater.Release);
	        this.downloading = source["downloading"];
	        this.downloadedPath = source["downloadedPath"];
	        this.skipped = source["skipped"];
	        this.checkedAt = source["checkedAt"];
	        this.error = source["error"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace media {
	
	export class Chapter {
	    title: string;
	    startTime: number;
	    endTime: number;
	
	    static createFrom(source: any = {}) {
	        return new Chapter(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.startTime = source["startTime"];
	        this.endTime = source["endTime"];
	    }
	}
	export class MediaCapabilities {
	    hasVideo: boolean;
	    hasAudio: boolean;
	    hasCombined: boolean;
	    hasSeparateVideo: boolean;
	    hasSeparateAudio: boolean;
	    hasSubtitles: boolean;
	    hasAutomaticCaptions: boolean;
	    hasChapters: boolean;
	    hasThumbnail: boolean;
	    hasDescription: boolean;
	    hasMultipleFormats: boolean;
	    isCollection: boolean;
	    isLive: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MediaCapabilities(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hasVideo = source["hasVideo"];
	        this.hasAudio = source["hasAudio"];
	        this.hasCombined = source["hasCombined"];
	        this.hasSeparateVideo = source["hasSeparateVideo"];
	        this.hasSeparateAudio = source["hasSeparateAudio"];
	        this.hasSubtitles = source["hasSubtitles"];
	        this.hasAutomaticCaptions = source["hasAutomaticCaptions"];
	        this.hasChapters = source["hasChapters"];
	        this.hasThumbnail = source["hasThumbnail"];
	        this.hasDescription = source["hasDescription"];
	        this.hasMultipleFormats = source["hasMultipleFormats"];
	        this.isCollection = source["isCollection"];
	        this.isLive = source["isLive"];
	    }
	}
	export class MediaFormat {
	    formatId: string;
	    extension: string;
	    width: number;
	    height: number;
	    fps: number;
	    videoCodec: string;
	    audioCodec: string;
	    rawVideoCodec: string;
	    rawAudioCodec: string;
	    videoBitrate: number;
	    audioBitrate: number;
	    totalBitrate: number;
	    fileSize: number;
	    fileSizeApprox: number;
	    hasVideo: boolean;
	    hasAudio: boolean;
	    dynamicRange: string;
	    protocol: string;
	    formatNote: string;
	    language: string;
	    kind: string;
	    qualityLabel: string;
	    resolution: string;
	    label: string;
	    sizeLabel: string;
	    recommended: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MediaFormat(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.formatId = source["formatId"];
	        this.extension = source["extension"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.fps = source["fps"];
	        this.videoCodec = source["videoCodec"];
	        this.audioCodec = source["audioCodec"];
	        this.rawVideoCodec = source["rawVideoCodec"];
	        this.rawAudioCodec = source["rawAudioCodec"];
	        this.videoBitrate = source["videoBitrate"];
	        this.audioBitrate = source["audioBitrate"];
	        this.totalBitrate = source["totalBitrate"];
	        this.fileSize = source["fileSize"];
	        this.fileSizeApprox = source["fileSizeApprox"];
	        this.hasVideo = source["hasVideo"];
	        this.hasAudio = source["hasAudio"];
	        this.dynamicRange = source["dynamicRange"];
	        this.protocol = source["protocol"];
	        this.formatNote = source["formatNote"];
	        this.language = source["language"];
	        this.kind = source["kind"];
	        this.qualityLabel = source["qualityLabel"];
	        this.resolution = source["resolution"];
	        this.label = source["label"];
	        this.sizeLabel = source["sizeLabel"];
	        this.recommended = source["recommended"];
	    }
	}
	export class MediaEntry {
	    index: number;
	    id: string;
	    title: string;
	    thumbnailUrl: string;
	    duration: number;
	    durationText: string;
	    webpageUrl: string;
	    contentType: string;
	    formats: MediaFormat[];
	    resolved: boolean;
	    unavailable: boolean;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new MediaEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.id = source["id"];
	        this.title = source["title"];
	        this.thumbnailUrl = source["thumbnailUrl"];
	        this.duration = source["duration"];
	        this.durationText = source["durationText"];
	        this.webpageUrl = source["webpageUrl"];
	        this.contentType = source["contentType"];
	        this.formats = this.convertValues(source["formats"], MediaFormat);
	        this.resolved = source["resolved"];
	        this.unavailable = source["unavailable"];
	        this.message = source["message"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class SubtitleLanguage {
	    code: string;
	    name: string;
	    automatic: boolean;
	    formats: string[];
	
	    static createFrom(source: any = {}) {
	        return new SubtitleLanguage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.name = source["name"];
	        this.automatic = source["automatic"];
	        this.formats = source["formats"];
	    }
	}
	export class MediaInfo {
	    platform: string;
	    platformSlug: string;
	    extractor: string;
	    extractorKey: string;
	    id: string;
	    title: string;
	    uploader: string;
	    channel: string;
	    description: string;
	    duration: number;
	    durationText: string;
	    thumbnailUrl: string;
	    webpageUrl: string;
	    requestedUrl: string;
	    contentType: string;
	    uploadDate: string;
	    viewCount: number;
	    liveStatus: string;
	    formats: MediaFormat[];
	    subtitles: SubtitleLanguage[];
	    chapters: Chapter[];
	    isCollection: boolean;
	    collectionTitle: string;
	    entries: MediaEntry[];
	    entriesResolved: boolean;
	    capabilities: MediaCapabilities;
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new MediaInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.platform = source["platform"];
	        this.platformSlug = source["platformSlug"];
	        this.extractor = source["extractor"];
	        this.extractorKey = source["extractorKey"];
	        this.id = source["id"];
	        this.title = source["title"];
	        this.uploader = source["uploader"];
	        this.channel = source["channel"];
	        this.description = source["description"];
	        this.duration = source["duration"];
	        this.durationText = source["durationText"];
	        this.thumbnailUrl = source["thumbnailUrl"];
	        this.webpageUrl = source["webpageUrl"];
	        this.requestedUrl = source["requestedUrl"];
	        this.contentType = source["contentType"];
	        this.uploadDate = source["uploadDate"];
	        this.viewCount = source["viewCount"];
	        this.liveStatus = source["liveStatus"];
	        this.formats = this.convertValues(source["formats"], MediaFormat);
	        this.subtitles = this.convertValues(source["subtitles"], SubtitleLanguage);
	        this.chapters = this.convertValues(source["chapters"], Chapter);
	        this.isCollection = source["isCollection"];
	        this.collectionTitle = source["collectionTitle"];
	        this.entries = this.convertValues(source["entries"], MediaEntry);
	        this.entriesResolved = source["entriesResolved"];
	        this.capabilities = this.convertValues(source["capabilities"], MediaCapabilities);
	        this.warnings = source["warnings"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ResolutionPreset {
	    label: string;
	    height: number;
	    available: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ResolutionPreset(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.height = source["height"];
	        this.available = source["available"];
	    }
	}

}

export namespace updater {
	
	export class Release {
	    version: string;
	    tagName: string;
	    name: string;
	    notes: string;
	    publishedAt: string;
	    pageUrl: string;
	    installer?: ghrelease.Asset;
	    checksums?: ghrelease.Asset;
	
	    static createFrom(source: any = {}) {
	        return new Release(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.tagName = source["tagName"];
	        this.name = source["name"];
	        this.notes = source["notes"];
	        this.publishedAt = source["publishedAt"];
	        this.pageUrl = source["pageUrl"];
	        this.installer = this.convertValues(source["installer"], ghrelease.Asset);
	        this.checksums = this.convertValues(source["checksums"], ghrelease.Asset);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace ytdlp {
	
	export class Error {
	    kind: string;
	    message: string;
	    hint?: string;
	    details?: string;
	
	    static createFrom(source: any = {}) {
	        return new Error(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.message = source["message"];
	        this.hint = source["hint"];
	        this.details = source["details"];
	    }
	}

}

