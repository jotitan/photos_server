import React, {useCallback, useEffect, useState} from 'react'
import {Badge, Col, Drawer, Empty, Input, Modal, notification, Popconfirm, Popover, Row, Tag, Tooltip} from 'antd'
import Gallery from 'react-grid-gallery'
import axios from "axios";
import {getBaseUrl, getBaseUrlHref} from "../treeFolder";
import SharePanel from "../share"
import './gallery.css'

import {
    ChromeOutlined,
    CloseOutlined,
    DeleteFilled,
    DeleteTwoTone,
    EditOutlined,
    FileImageOutlined,
    FilterOutlined,
    PictureOutlined,
    PlusCircleOutlined,
    PlusOutlined,
    ReloadOutlined,
    SaveOutlined,
    ShareAltOutlined,
    UserAddOutlined,
    FullscreenOutlined
} from "@ant-design/icons";
import {TransformComponent, TransformWrapper} from "react-zoom-pan-pinch";

import {CirclePicker} from 'react-color';
import Timeline from "../../components/timeline";
import {useTranslation} from "react-i18next";
import EditTitle from "../../components/edit-title";


const setProperty = (ctx, property, value) => {
    const copy = {...ctx};
    copy[property] = value;
    return copy;
}

const adaptImages = photos => {
    return photos
        .filter(file => file.ImageLink != null)
        .sort((img1, img2) => new Date(img1.Date) - new Date(img2.Date))
        .map(img => {
            let d = new Date(img.Date).toLocaleString();
            let folder = img.HdLink.replace(img.Name, '').replace('/imagehd/', '');
            return {
                hdLink: getBaseUrlHref() + img.HdLink,
                path: img.HdLink,
                folder: folder,
                Date: d,
                caption: "", thumbnail: getBaseUrl() + img.ThumbnailLink, src: getBaseUrl() + img.ImageLink,
                customOverlay: <div style={{
                    padding: 2 + 'px',
                    bottom: 0,
                    opacity: 0.8,
                    fontSize: 10 + 'px',
                    position: 'absolute',
                    backgroundColor: 'white'
                }}>{d}</div>,
                thumbnailWidth: img.Width || 0,
                thumbnailHeight: img.Height || 0
            }
        });
};

class mode {
    setImages;
    context;
    setContext;
    t;

    constructor(setImages, context, setContext, t) {
        this.setImages = setImages;
        this.context = context;
        this.setContext = setContext;
        this.t = t;
    }

    defaultSelect(index) {
        this.setImages(list => {
            let copy = list.slice();
            copy[index].isSelected = list[index].isSelected != null ? !list[index].isSelected : true;
            return copy;
        });
    }

    reset() {
        console.log("not implemented")
    }

    select(index) {
        console.log("not implemented")
    }

    showFullMenu() {
        return true;
    }

    getContent() {
    }
}

class deleteMode extends mode {
    select(index) {
        this.defaultSelect(index);
    }
}

class tagMode extends mode {
    name;
    count = 0;
    paths = {};

    reset() {
        this.setContext(ctx => {
            const copy = {...ctx};
            copy.currentTag = null;
            copy.paths = null;
            return copy;
        })
    }

    select(index, image) {
        this.setContext(ctx => {
            if (ctx.currentTag == null) {
                return ctx;
            }
            let copy = {...ctx};
            if (copy.paths == null) {
                copy.paths = {};
            }
            let pathList = copy.paths[copy.currentTag.id];
            if (pathList == null) {
                pathList = [];
                copy.paths[copy.currentTag.id] = pathList;
            }
            // If index already exist, remove it
            let found = pathList.findIndex(p => p.idx === index)
            if (found !== -1) {
                // Remove
                pathList.splice(found, 1);
            } else {
                // Check if already exists in original list, if not, add to remove
                if (ctx.originalPaths != null
                    && ctx.originalPaths[copy.currentTag.id] != null
                    && ctx.originalPaths[copy.currentTag.id].some(p => image.path.indexOf(p) !== -1)) {
                    // If already exist in original path, try to remove it
                    pathList.push({path: image.path, idx: index, delete: true})
                } else {
                    pathList.push({path: image.path, idx: index, delete: false})
                }
            }
            this.defaultSelect(index);
            return copy;
        })
    }

    setName(name) {
        this.name = name;
    }

    showFullMenu() {
        return false;
    }

    selectPeople(tag) {
        this.setContext(ctx => {
            let copy = {...ctx};
            copy.currentTag = tag;
            // Select only images of people
            this.setImages(list => {
                let copyImages = list.slice();
                copyImages.forEach(i => i.isSelected = false);
                if (copy.paths != null && copy.paths[copy.currentTag.id] != null) {
                    copy.paths[copy.currentTag.id].forEach(p => copyImages[p.idx].isSelected = true)
                }
                if (copy.originalPaths != null && copy.originalPaths[copy.currentTag.id] != null) {
                    copy.originalPaths[copy.currentTag.id].forEach(p => {
                        copyImages.find(img => img.path.indexOf(p) !== -1).isSelected = true
                    })
                }
                return copyImages;
            })
            return copy;
        });

    }

    countTaged(people, context) {
        // Count selected path and originalPaths
        let count = 0;
        if (context.paths != null && context.paths[people.id] != null) {
            count += context.paths[people.id].map(p => p.delete === true ? -1 : 1).reduce((total, v) => total + v, 0);
        }
        if (context.originalPaths != null && context.originalPaths[people.id] != null) {
            count += context.originalPaths[people.id].length;
        }
        return count;
    }

    updateNewPeople(value) {
        let name = value.target.value;
        if (value.key === 'Enter') {
            // Save
            axios({
                method: 'POST',
                url: `${getBaseUrl()}/tag/add_people?name=${name}`,
            }).then(r => {
                notification["success"]({message: 'New people added', description: `New people ${name} well added`})
                this.setContext(ctx => {
                    const copy = {...ctx};
                    copy.peoples.push({name: name, id: r.data});
                    copy.flag = false;
                    return copy;
                });
            })

        }
    }

    save(context) {
        // Request is [{tag,folder,paths:[]}]
        const data = Object.keys(context.paths).map(tag => {
            // Keep only last part of path
            return {
                paths: context.paths[tag].filter(p => !p.delete).map(v => v.path.substr(v.path.lastIndexOf("/") + 1)),
                deleted: context.paths[tag].filter(p => p.delete).map(v => v.path.substr(v.path.lastIndexOf("/") + 1)),
                tag: parseInt(tag),
                folder: context.id
            };
        })
        return axios({
            method: 'POST',
            url: `${getBaseUrl()}/tag/tag_folder`,
            data: data
        }).then(() => notification["success"]({message: 'Tags saved', description: `All tagged have been saved`}));
    }

    // return peoples
    getContent(context) {
        return <>
            {context.peoples != null ? context.peoples.map(p =>
                <p
                    onClick={() => this.selectPeople(p)}
                    className={`people${context.currentTag != null && context.currentTag.id === p.id ? " selected" : ""}`}>
                    {p.name} <Badge overflowCount={1000} count={this.countTaged(p, context)}
                                    style={{backgroundColor: '#427a10'}}/>
                </p>) : ''}
            <Tag style={{cursor: 'pointer'}} onClick={() => this.setContext(ctx => setProperty(ctx, "flag", true))}>
                <UserAddOutlined/>+ {this.t('people.add')}
            </Tag>
            <Tag style={{cursor: 'pointer'}} onClick={() => this.save(context)}><SaveOutlined/> {this.t('people.save')}
            </Tag>
            {context.flag ?
                <p>
                    People :
                    <Input onKeyUp={v => this.updateNewPeople(v)}/>
                </p> : <></>}
        </>
    }

    asSet(list) {
        return new Set(list);
    }

    filterPeople(people, context) {
        // Already select, show all images
        if (people.id === context.currentTag) {
            this.setContext(ctx => setProperty(ctx, "currentTag", null))
            return this.setImages(context.allImages);
        }
        // If image already filtered, restore original images
        axios({
            method: 'GET',
            url: `${getBaseUrl()}/tag/search?folder=${context.id}&tag=${people.id}`
        })
            .then(d => {
                this.setImages(() => {
                    // Save current tag and original images
                    this.setContext(ctx => {
                        const copy = {...ctx};
                        copy.currentTag = people.id;
                        return copy;
                    })
                    let s = this.asSet(d.data);
                    // Keep only images in list where path exists in returned list
                    return context.allImages.filter(i => s.has(i.path.substr(i.path.lastIndexOf("/") + 1)))
                })
            })
    }

    getFilterContent(context) {
        return <>
            {context.peoples != null ? context.peoples.map(p =>
                <p key={"filter-people"}
                   onClick={() => this.filterPeople(p, context)}
                   className={`people${context.currentTag === p.id ? " selected" : ""}`}>
                    {p.name}
                </p>) : ''}
        </>
    }
}

const loadTagsOfFolder = id => {
    return axios({
        method: 'GET',
        url: `${getBaseUrl()}/tag/search_folder?folder=${id}`,
    });
}

const loadPeoplesTag = () => {
    return axios({
        method: 'GET',
        url: getBaseUrl() + '/tag/peoples',
    })
}

// setIsAddFolderPanelVisible to show folder to upload
export default function MyGallery({
                                      setUrlFolder,
                                      urlFolder,
                                      refresh,
                                      titleGallery,
                                      canAdmin,
                                      setIsAddFolderPanelVisible,
                                      setCurrentFolder,
                                      update,
                                      setUpdate,
                                      setCollapsed
                                  }) {
    const [images, setImages] = useState([]);
    const [originalImages, setOriginalImages] = useState([]);
    const [imageToZoom, setImageToZoom] = useState('');
    const [zoomEnable, setZoomEnable] = useState(false);
    //const [updateUrl, setUpdateUrl] = useState('');
    const [showSharePanel, setShowSharePanel] = useState(false);
    const [updateExifUrl, setUpdateExifUrl] = useState('');
    const [details, setDetails] = useState('');
    const [removeFolderUrl, setRemoveFolderUrl] = useState('');
    const [currentImage, setCurrentImage] = useState(-1);
    const [updateRunning, setUpdateRunning] = useState(false);
    const [updateExifRunning, setUpdateExifRunning] = useState(false);
    const [key, setKey] = useState(-1);
    const [lightboxVisible, setLightboxVisible] = useState(false);
    const [showThumbnails, setShowThumbnails] = useState(false);
    const [comp, setComp] = useState(null);
    const [showTimeline, setShowTimeline] = useState(false);
    const [editDetails, setEditDetails] = useState(false);
    const {t} = useTranslation();

    const [input, setInput] = useState('');
    const [contextSelect, setContextSelect] = useState({input: input, setInput: setInput, flag: false});
    const dMode = new deleteMode(setImages, contextSelect, setContextSelect, t);
    const tMode = new tagMode(setImages, contextSelect, setContextSelect, t);
    const [selectMode, setSelectMode] = useState(dMode)

    const [filterEnable, setFilterEnable] = useState(false);

    let baseUrl = getBaseUrl();
    // Gestion du tag
    const [showInputTag, setShowInputTag] = useState(false);
    const [tags, setTags] = useState([]);
    const [nextTagValue, setNextTagValue] = useState('');

    useEffect(() => {
        if (comp != null) {
            setTimeout(() => comp.onResize(), 300);
        }
    }, [refresh, comp])

    useEffect(() => {
        loadPeoplesTag().then(data => setContextSelect(ctx => setProperty(ctx, "peoples", data.data)))
        if (canAdmin) {
            window.addEventListener('keydown', e => {
                if (e.key === "t") {
                    // Switch thumbnail
                    setShowThumbnails(s => !s);
                }
                setKey(e.key)
            });
        }
    }, [canAdmin, setShowThumbnails]);


    useEffect(() => {
        if (lightboxVisible && key === "Delete") {
            images[currentImage].isSelected = !images[currentImage].isSelected;
            setKey("");
        }
    }, [currentImage, key, lightboxVisible, images]);

    const saveTag = (tag, callback) => {
        axios({
            method: 'POST',
            url: urlFolder.tags,
            data: JSON.stringify(tag),
        }).then(callback);
    };

    // Check if pictures contains many foldesr
    const isMultipleFolders = images => images.filter(img => img.HdLink).map(img => img.HdLink.replace(img.Name, '')).reduce((s, value) => s.add(value), new Set()).size > 1;

    const memLoadImages = useCallback(() => {
        if (urlFolder === '' || urlFolder.load === '') {
            return;
        }
        axios({
            method: 'GET',
            url: urlFolder.load,
        }).then(d => {

            // Filter image by time before
            setDetails(d.data);
            //setUpdateUrl(d.data.UpdateUrl);
            setUpdateExifUrl(d.data.UpdateExifUrl);
            setRemoveFolderUrl(d.data.RemoveFolderUrl);
            setCurrentFolder(d.data.FolderPath);
            setContextSelect(ctx => setProperty(ctx, "id", d.data.Id))
            loadTagsOfFolder(d.data.Id).then(data => setContextSelect(ctx => setProperty(ctx, "originalPaths", data.data)))
            let photos = d.data.Files != null ? d.data.Files : d.data;
            setShowTimeline(isMultipleFolders(photos))
            setTags(d.data.Tags.map(t => {
                return {value: t.Value, color: t.Color}
            }));
            let p = adaptImages(photos)
            setImages(() => {
                setOriginalImages(p);
                setContextSelect(ctx => setProperty(ctx, "allImages", p))
                return isMultipleFolders(photos) ? [] : p
            });
        })
    }, [urlFolder, setCurrentFolder]);

    useEffect(() => memLoadImages(urlFolder), [urlFolder, memLoadImages, update]);

    const selectImage = index => {
        selectMode.select(index, images[index])
    };

    const removeFolder = () => {
        axios({
            method: 'DELETE',
            url: `${baseUrl}${removeFolderUrl}`
        }).then(r => {
            if (r.data === 'success') {
                notification["success"]({message: "Succès", description: `Le répertoire a été bien supprimé`});
                setDetails(t => {
                    const copy = [...t];
                    copy.UpdateUrl = '';
                    return copy;
                })
                //setUpdateUrl('');
                setUrlFolder({load: '', tags: ''});
                setUpdate(!update);
            }
        });
    };

    const deleteSelection = () => {
        axios({
            method: 'POST',
            url: `${baseUrl}/delete`,
            data: JSON.stringify(images.filter(i => i.isSelected).map(i => i.path))
        }).then(r => {
            if (r.data.errors === 0) {
                let count = images.filter(i => i.isSelected).length;
                setImages(images.filter(i => !i.isSelected));
                notification["success"]({message: "Succès", description: `${count} images ont été bien supprimées`});
            }
        });
    };

    const updateFolder = () => {
        if (canAdmin && details.UpdateUrl !== "") {
            setUpdateRunning(true);
            axios({
                method: 'POST',
                url: `${baseUrl}${details.UpdateUrl}`,
            }).then(() => {
                // Reload folder
                memLoadImages(urlFolder);
                setUpdateRunning(false);
            })
        }
    };

    const updateExifFolder = () => {
        if (canAdmin && updateExifUrl !== "") {
            setUpdateExifRunning(true);
            axios({
                method: 'GET',
                url: `${baseUrl}${updateExifUrl}`,
            }).then(() => {
                // Reload folder
                memLoadImages(urlFolder);
                setUpdateExifRunning(false);
            })
        }
    };

    // Show informations about selected images
    const showSelected = () => {
        const selected = images.filter(i => i.isSelected).length;
        return selected > 0 && selectMode.showFullMenu() && !filterEnable ? <>
            <Popconfirm placement="bottom" title={"Es tu sûr de vouloir supprimer ces photos"}
                        onConfirm={deleteSelection} okText="Oui" cancelText="Non">
                <Tooltip key={"delete-image-info"} placement="top" title={"Supprimer la sélection"}
                         overlayStyle={{zIndex: 20000}}>
                    <DeleteFilled className={"button"}/>
                </Tooltip>
                <span style={{marginLeft: 10 + 'px'}}>{selected}</span>
            </Popconfirm>
        </> : ''
    };

    const addPhotosToFolder = () => {
        setIsAddFolderPanelVisible(true);
    };

    const resetSelectedImage = () => {
        selectMode.reset();
        setImages(list => list.map(i => {
                i.isSelected = false;
                return i;
            })
        )
    }

    const showUpdateLink = () => {
        return !canAdmin || details.UpdateUrl === '' || details.UpdateUrl == null ? <></> :
            selectMode.showFullMenu() ?
                <>
                    <Tooltip key={"image-fullscreen"} placement="top" title={"Fullscreen"}>
                        <FullscreenOutlined onClick={setCollapsed} className={"button"}/>
                    </Tooltip>
                    <Tooltip key={"image-share"} placement="top" title={"Partager le répertoire"}>
                        <ShareAltOutlined style={{marginLeft:10}} onClick={() => setShowSharePanel(true)} className={"button"}/>
                    </Tooltip>
                    {isFolderEmpty() && !filterEnable ?
                        <Popconfirm placement="bottom" title={"Es tu sûr de vouloir supprimer ce répertoire vide"}
                                    onConfirm={removeFolder} okText="Oui" cancelText="Non">
                            <Tooltip key={"delete-folder"} placement="top" title={"Supprimer le répertoire"}>
                                <DeleteTwoTone style={{cursor: 'pointer', padding: '4px', backgroundColor: '#ff8181'}}
                                               twoToneColor={"#b32727"}/>
                            </Tooltip>
                        </Popconfirm> : <></>}
                    <Popconfirm placement="bottom" title={"Es tu sûr de vouloir mettre à jour les Exifs"}
                                onConfirm={updateExifFolder} okText="Oui" cancelText="Non">
                        <Tooltip key={"update-exif"} placement="top" title={"Mettre à jour les Exifs"}>
                            <ChromeOutlined style={{marginLeft: 10}} spin={updateExifRunning} className={"button"}/>
                        </Tooltip>
                    </Popconfirm>
                    <Popconfirm placement="bottom" title={"Es tu sûr de vouloir mettre à jour le répertoire"}
                                onConfirm={updateFolder} okText="Oui" cancelText="Non">
                        <Tooltip key={"update-folder2"} placement="top" title={"Mettre à jour le répertoire"}>
                            <ReloadOutlined style={{marginLeft: 10}} spin={updateRunning} className={"button"}/>
                        </Tooltip>
                    </Popconfirm>
                    <Tooltip key={"add-image-info"} placement="top" title={"Ajouter des photos"}>
                        <PlusCircleOutlined className={"button"} style={{marginLeft: 10}} onClick={addPhotosToFolder}/>
                    </Tooltip>
                    <Tooltip key={"tag-images"} placement="top" title={"Tagger des photos"}>
                        <UserAddOutlined className={"button"} style={{marginLeft: 10}} onClick={() => {
                            loadTagsOfFolder(contextSelect.id).then(data => setContextSelect(ctx => setProperty(ctx, "originalPaths", data.data)))
                            setSelectMode(tMode)
                        }}/>
                    </Tooltip>
                    <Tooltip key={"filter-image"} placement="top" title={"Filter"}>
                        <FilterOutlined className={"button"}
                                        style={{marginLeft: 10, backgroundColor: filterEnable ? 'green' : ''}}
                                        onClick={() => {
                                            if (filterEnable) {
                                                setImages(() => contextSelect.allImages);
                                                setContextSelect(ctx => setProperty(ctx, "currentTag", null));
                                            }
                                            setFilterEnable(v => !v)
                                        }}/>
                    </Tooltip>
                </> : <>
                    <Tag color="gray" style={{cursor: 'pointer'}} onClick={() => {
                        resetSelectedImage();
                        setSelectMode(dMode)
                    }}>{t('people.close')}</Tag>
                </>;
    };

    const showTagsBloc = () => {
        return (
            <>{tags
                .sort((a, b) => a.value < b.value ? -1 : 1)
                .map(t =>
                    <Tooltip key={`tp${t.value}`} trigger={"click"} title={
                        <CirclePicker width={'250px'} onChange={color => updateColor(color, t)} circleSize={26}
                                      circleSpacing={8}/>
                    }><Tag key={t.value} color={t.color} closable={true} onClose={() => removeTag(t)}>{t.value}</Tag>
                    </Tooltip>
                )}
                {!showInputTag && canAdmin && urlFolder.load !== '' ?
                    <Tag color="gray" onClick={() => setShowInputTag(true)}><PlusOutlined/> tag</Tag> : <></>}
                {showInputTag ?
                    <Input size={"small"} style={{width: 78 + 'px'}} onKeyUp={updateText} autoFocus={true}/> : <></>}
            </>);
    }

    const updateText = value => {
        switch (value.key) {
            case 'Enter':
                let tag = {value: nextTagValue, color: 'green'};
                setShowInputTag(false);
                setNextTagValue('');
                saveTag(tag, () => setTags(list => [...list, tag]));
                break;
            default:
                setNextTagValue(value.target.value);
        }
    };

    const updateColor = (color, tag) => {
        let newTag = {value: tag.value, color: color.hex};
        saveTag(newTag, () => setTags(tgs => [...tgs.filter(n => n.value !== tag.value), newTag]))
    };

    const removeTag = tag => saveTag({Value: tag.value, Color: tag.color, ToRemove: true});

    // Add behaviour when show image in lightbox
    const getCustomActions = () => {
        return [
            <div style={{paddingTop: 5 + 'px'}} key={"detail-lightbox"}>
                {images != null && currentImage !== -1 && images[currentImage].isSelected ?
                    <DeleteTwoTone twoToneColor={"red"} style={{color: 'red', fontSize: 22 + 'px'}}/> : ''}
                <Tooltip key={"download-image-info"} placement="top" title={"Télécharger en HD"}
                         overlayStyle={{zIndex: 20000}}>
                    <a target={"_blank"} rel="noopener noreferrer"
                       download={images != null && currentImage !== -1 ? images[currentImage].Name : ''}
                       href={images != null && currentImage !== -1 ? images[currentImage].hdLink : ''}>
                        <FileImageOutlined style={{color: 'white', fontSize: 22 + 'px'}}/>
                    </a>
                </Tooltip>
                <Tooltip key={"zoom-image-info"} placement="top" title={"Zoom"} overlayStyle={{zIndex: 20000}}>
                    <PlusCircleOutlined style={{color: 'white', fontSize: 22 + 'px', marginLeft: 5}}
                                        onClick={() => setZoomEnable(true)}/>
                </Tooltip>
                <span style={{color: 'white', paddingLeft: 20 + 'px'}}>
                   {images != null && currentImage !== -1 ? images[currentImage].Date : ''}
                    {images != null && currentImage !== -1 ? ' - ' + images[currentImage].folder : ''}
               </span>
                {/* Show peoples on image ?*/}
            </div>
        ]
    };

    const showGallery = () => {
        return <>
            <Gallery ref={setComp}
                     images={images}
                     showImageCount={false}
                     lightboxWillClose={() => setLightboxVisible(false)}
                     lightboxWillOpen={() => setLightboxVisible(true)}
                     onSelectImage={selectImage}
                     enableImageSelection={canAdmin === true}
                     currentImageWillChange={indexImage => {
                         setCurrentImage(indexImage)
                         setImageToZoom(images[indexImage].hdLink);
                     }}
                     customControls={getCustomActions()}
                     showLightboxThumbnails={showThumbnails}
                     backdropClosesModal={true} lightboxWidth={2000}/>
        </>
    };

    const isFolderEmpty = () => {
        return urlFolder !== "" && urlFolder.load !== "" && images.length === 0;
    };

    const showEmptyMessage = () => {
        return (
            urlFolder === '' || urlFolder.load === '' ? <></> :
                <Empty style={{marginTop: '40vh'}}
                       description={<span style={{color: 'white', fontWeight: 'bold'}}>Pas de photos</span>}
                       image={Empty.PRESENTED_IMAGE_SIMPLE}/>
        );
    };

    const showPeopleTagDrawer = () => {
        return <Drawer
            title={t('people.title-panel')}
            placement="right"
            closable={false}
            mask={false}
            width={'12%'}
            visible={!selectMode.showFullMenu()}
        >
            {selectMode.getContent(contextSelect)}
        </Drawer>
    }

    const showFilterTagDrawer = () => {
        return <Drawer
            title="Filter people"
            placement="right"
            closable={false}
            mask={false}
            width={'12%'}
            visible={filterEnable}
        >
            {tMode.getFilterContent(contextSelect)}
        </Drawer>
    }

    const showTitle = () => {
        if (details.Title && details.Title !== '') {
            return <Popover title={<span>{details.Title} {canAdmin ?
                <EditOutlined onClick={() => setEditDetails(true)}/> : ''}</span>}
                            content={<pre>{details.Description}</pre>}><span
                style={{cursor: 'pointer'}}>{details.Title}</span></Popover>
        }
        return <Popover
            title={<span>{titleGallery} {canAdmin ? <EditOutlined onClick={() => setEditDetails(true)}/> : ''}</span>}
            content={'Ajouter un titre et une description'}><span
            style={{cursor: 'pointer'}}>{titleGallery}</span></Popover>
    }

    const [scale, setScale] = useState(1);
    const [posX, setPosX] = useState(1);
    const [posY, setPosY] = useState(1);
    return (
        <>
            <Row className={"options"}>
                <Col flex={"300px"}>
                    {showTitle()}
                    <span style={{paddingLeft: 5, paddingRight: 5}}>-</span>
                    {images.length} <PictureOutlined/>
                </Col>
                <Col flex={"100px"}>
                    {showSelected()}
                </Col>
                <Col key="update-link" flex={"230px"}>
                    {showUpdateLink()}
                </Col>
                <Col flex={"auto"}>{showTagsBloc()}</Col>
            </Row>

            {showTimeline ?
                <Row>
                    <Col flex={`${selectMode.showFullMenu() && !filterEnable ? '100%' : '85%'}`}
                         style={{marginTop: 35 + 'px', backgroundColor: 'rgb(0,21,41)'}}>
                        <Timeline images={originalImages} setImages={imgs => {
                            setImages(() => {
                                setContextSelect(ctx => setProperty(ctx, "allImages", imgs))
                                return imgs
                            })
                        }}/>
                    </Col>
                </Row> : <></>}
            {canAdmin && editDetails ?
                <EditTitle folder={urlFolder.path} initialValues={details} close={() => setEditDetails(false)}/> : ''}
            <Row className={"gallery"}>
                <Col flex={`${selectMode.showFullMenu() && !filterEnable ? '100%' : '85%'}`}
                     style={{marginTop: `${showTimeline ? '30' : '36'}px`}}>
                    {images.length === 0 ? showEmptyMessage() : showGallery()}
                </Col>
                <Modal visible={zoomEnable}
                       onCancel={() => {
                           setScale(1);
                           setPosX(0);
                           setPosY(0);
                           setZoomEnable(false)
                       }}
                       width={90 + '%'}
                       style={{top: 20}}
                       footer={[]}
                       closeIcon={<CloseOutlined style={{color: 'white', fontSize: 20}}/>}
                       wrapClassName={"modal-zoom"}>
                    <TransformWrapper scale={scale} positionX={posX} positionY={posY}>
                        <TransformComponent>
                            <img src={imageToZoom} alt="Zoomed panel"/>
                        </TransformComponent>
                    </TransformWrapper>
                </Modal>
                <SharePanel showSharePanel={showSharePanel} hide={() => setShowSharePanel(false)}
                            path={urlFolder.path}/>
                {showPeopleTagDrawer()}
                {showFilterTagDrawer()}
            </Row>
        </>
    )
}
