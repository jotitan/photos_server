import React, {useCallback, useEffect, useState} from 'react'
import {Col, Empty, Input, Modal, notification, Popconfirm, Row, Tag, Tooltip} from 'antd'
import Gallery from 'react-grid-gallery'
import axios from "axios";
import {getBaseUrl, getBaseUrlHref} from "../treeFolder";
import SharePanel from "../share"


import {
    UserAddOutlined,
    ChromeOutlined,
    CloseOutlined,
    DeleteFilled,
    DeleteTwoTone,
    FileImageOutlined,
    PictureOutlined,
    PlusCircleOutlined,
    PlusOutlined,
    ReloadOutlined
} from "@ant-design/icons";
import {TransformComponent, TransformWrapper} from "react-zoom-pan-pinch";


import {CirclePicker} from 'react-color';

const adaptImages = photos=> {
    return photos
        .filter(file=>file.ImageLink != null)
        .sort((img1,img2)=>new Date(img1.Date) - new Date(img2.Date))
        .map(img=>{
            let d = new Date(img.Date).toLocaleString();
            let folder = img.HdLink.replace(img.Name,'').replace('/imagehd/','');
            return {
                hdLink:getBaseUrlHref() + img.HdLink,
                path:img.HdLink,
                folder:folder,
                Date:d,
                caption:"",thumbnail:getBaseUrl() + img.ThumbnailLink,src:getBaseUrl() + img.ImageLink,
                customOverlay:<div style={{padding:2+'px',bottom:0,opacity:0.8,fontSize:10+'px',position:'absolute',backgroundColor:'white'}}>{d}</div>,
                thumbnailWidth:img.Width,
                thumbnailHeight:img.Height
            }
        });
};

// setIsAddFolderPanelVisible to show folder to upload
export default function MyGallery({setUrlFolder,urlFolder,refresh,titleGallery,canAdmin,setIsAddFolderPanelVisible,setCurrentFolder,update,setUpdate}) {
    const [images,setImages] = useState([]);
    const [imageToZoom,setImageToZoom] = useState('');
    const [zoomEnable,setZoomEnable] = useState(false);
    const [updateUrl,setUpdateUrl] = useState('');
    const [showSharePanel,setShowSharePanel] = useState(false);
    const [updateExifUrl,setUpdateExifUrl] = useState('');
    const [removeFolderUrl,setRemoveFolderUrl] = useState('');
    const [currentImage,setCurrentImage] = useState(-1);
    const [updateRunning,setUpdateRunning] = useState(false);
    const [updateExifRunning,setUpdateExifRunning] = useState(false);
    const [key,setKey] = useState(-1);
    const [lightboxVisible,setLightboxVisible] = useState(false);
    const [showThumbnails,setShowThumbnails] = useState(false);
    const [comp,setComp] = useState(null);
    let baseUrl = getBaseUrl();

    // Gestion du tag
    const [showInputTag,setShowInputTag] = useState(false);
    const [tags,setTags] = useState([]);
    const [nextTagValue,setNextTagValue] = useState('');

    useEffect(()=>{
        if(comp!=null){
            setTimeout(()=>comp.onResize(),300);
        }
    },[refresh,comp])

    useEffect(()=>{
        if(canAdmin){
            window.addEventListener('keydown',e=>{
                if(e.key === "t"){
                    // Switch thumbnail
                    setShowThumbnails(s=> !s);
                }
                setKey(e.key)
            });
        }
    },[canAdmin,setShowThumbnails]);

    useEffect(()=>{
        if(lightboxVisible && key === "Delete"){
            images[currentImage].isSelected=!images[currentImage].isSelected;
            setKey("");
        }
    },[currentImage,key,lightboxVisible,images]);

    const saveTag = (tag,callback) => {
        axios({
            method:'POST',
            url:urlFolder.tags,
            data:JSON.stringify(tag),
        }).then(callback);
    };

    const memLoadImages = useCallback(()=> {
        if(urlFolder === '' || urlFolder.load === ''){return;}
        axios({
            method:'GET',
            url:urlFolder.load,
        }).then(d=>{
            // Filter image by time before
            setUpdateUrl(d.data.UpdateUrl);
            setUpdateExifUrl(d.data.UpdateExifUrl);
            setRemoveFolderUrl(d.data.RemoveFolderUrl);
            setCurrentFolder(d.data.FolderPath);
            let photos = d.data.Files != null ? d.data.Files:d.data;
            setTags(d.data.Tags.map(t=>{return {value:t.Value,color:t.Color}}));
            setImages(adaptImages(photos));
        })
    },[urlFolder,setCurrentFolder]);

    useEffect(()=>memLoadImages(urlFolder),[urlFolder,memLoadImages,update]);

    const selectImage = index=>{
        setImages(list=>{
            let copy = list.slice();
            copy[index].isSelected = list[index].isSelected != null ? !list[index].isSelected : true;
            return copy;
        });
    };

    const removeFolder = ()=> {
        axios({
            method:'DELETE',
            url:baseUrl + removeFolderUrl
        }).then(r=>{
            if(r.data === 'success') {
                notification["success"]({message:"Succès",description:`Le répertoire a été bien supprimé`});
                setUpdateUrl('');
                setUrlFolder({load:'',tags:''});
                setUpdate(!update);
            }
        });
    };

    const deleteSelection = ()=>{
        axios({
            method:'POST',
            url:baseUrl + '/delete',
            data:JSON.stringify(images.filter(i=>i.isSelected).map(i=>i.path))
        }).then(r=>{
            if(r.data.errors === 0) {
                let count = images.filter(i => i.isSelected).length;
                setImages(images.filter(i => !i.isSelected));
                notification["success"]({message:"Succès",description:`${count} images ont été bien supprimées`});
            }
        });
    };

    const updateFolder = ()=> {
        if(canAdmin && updateUrl !==""){
            setUpdateRunning(true);
            axios({
                method:'GET',
                url:baseUrl + updateUrl,
            }).then(()=>{
                // Reload folder
                memLoadImages(urlFolder);
                setUpdateRunning(false);
            })
        }
    };

    const updateExifFolder = ()=> {
        if(canAdmin && updateExifUrl !==""){
            setUpdateExifRunning(true);
            axios({
                method:'GET',
                url:baseUrl + updateExifUrl,
            }).then(()=>{
                // Reload folder
                memLoadImages(urlFolder);
                setUpdateExifRunning(false);
            })
        }
    };

    // Show informations about selected images
    const showSelected = ()=>{
        const selected = images.filter(i=>i.isSelected).length;
        return selected > 0 ? <>
            <Popconfirm placement="bottom" title={"Es tu sûr de vouloir supprimer ces photos"}
                        onConfirm={deleteSelection} okText="Oui" cancelText="Non">
                <Tooltip key={"image-info"} placement="top" title={"Supprimer la sélection"} overlayStyle={{zIndex:20000}}>
                    <DeleteFilled className={"button"}/>
                </Tooltip>
                <span style={{marginLeft:10+'px'}}>{selected}</span>
            </Popconfirm>
        </>:''
    };

    const addPhotosToFolder = ()=> {
        setIsAddFolderPanelVisible(true);
    };

    const showUpdateLink = ()=> {
        return !canAdmin || updateUrl === '' || updateUrl ==null ? <></> :
            <>
                <Tooltip key={"image-share"} placement="top" title={"Partager le répertoire"}>
                    <UserAddOutlined onClick={()=>setShowSharePanel(true)} className={"button"}/>
                </Tooltip>
                {isFolderEmpty() ? <Popconfirm placement="bottom" title={"Es tu sûr de vouloir supprimer ce répertoire vide"}
                            onConfirm={removeFolder} okText="Oui" cancelText="Non">
                    <Tooltip key={"image-info"} placement="top" title={"Supprimer le répertoire"}>
                        <DeleteTwoTone style={{cursor:'pointer',padding:'4px',backgroundColor:'#ff8181'}} twoToneColor={"#b32727"}/>
                    </Tooltip>
                </Popconfirm>:<></>}
                <Popconfirm placement="bottom" title={"Es tu sûr de vouloir mettre à jour les Exifs"}
                            onConfirm={updateExifFolder} okText="Oui" cancelText="Non">
                    <Tooltip key={"image-info"} placement="top" title={"Mettre à jour les Exifs"}>
                        <ChromeOutlined style={{marginLeft:10}} spin={updateExifRunning} className={"button"}/>
                    </Tooltip>
                </Popconfirm>
                <Popconfirm placement="bottom" title={"Es tu sûr de vouloir mettre à jour le répertoire"}
                            onConfirm={updateFolder} okText="Oui" cancelText="Non">
                    <Tooltip key={"image-info"} placement="top" title={"Mettre à jour le répertoire"}>
                        <ReloadOutlined style={{marginLeft:10}} spin={updateRunning} className={"button"}/>
                    </Tooltip>
                </Popconfirm>
                <Tooltip key={"image-info"} placement="top" title={"Ajouter des photos"}>
                    <PlusCircleOutlined className={"button"} style={{marginLeft:10}} onClick={addPhotosToFolder}/>
                </Tooltip>
            </>;
    };

    const updateText = value=>{
        switch(value.key){
            case 'Enter':
                let tag = {value:nextTagValue,color:'green'};
                setShowInputTag(false);
                setNextTagValue('');
                saveTag(tag,()=>setTags(list=>[...list,tag]));
                break;
            default:
                setNextTagValue(value.target.value);
        }
    };

    const updateColor = (color,tag)=>{
        let newTag = {value:tag.value,color:color.hex};
        saveTag(newTag,()=>setTags(tags=>[...tags.filter(n=>n.value !== tag.value),newTag]))
    };

    const removeTag = (tag)=>{
        saveTag({Value:tag.value,Color:tag.color,ToRemove:true});
    };

    // Add behaviour when show image in lightbox
    const getCustomActions = ()=> {
        return [
            <div style={{paddingTop:5+'px'}} key={"detail-lightbox"}>
                {images!=null && currentImage !== -1 && images[currentImage].isSelected? <DeleteTwoTone twoToneColor={"red"} style={{color:'red',fontSize:22+'px'}} />:''}
                <Tooltip key={"image-info"} placement="top" title={"Télécharger en HD"} overlayStyle={{zIndex:20000}}>
                    <a target={"_blank"} rel="noopener noreferrer"
                       download={images != null && currentImage !== -1 ? images[currentImage].Name:''}
                       href={images != null && currentImage !== -1 ? images[currentImage].hdLink:''} >
                        <FileImageOutlined style={{color:'white',fontSize:22+'px'}}/>
                    </a>
                </Tooltip>
                <Tooltip key={"image-info"} placement="top" title={"Zoom"} overlayStyle={{zIndex:20000}}>
                    <PlusCircleOutlined style={{color:'white',fontSize:22+'px',marginLeft:5}} onClick={()=>setZoomEnable(true)}/>
                </Tooltip>
                <span style={{color:'white',paddingLeft:20+'px'}}>
                   {images!=null && currentImage!==-1 ? images[currentImage].Date:''}
                    {images!=null && currentImage!==-1 ? ' - ' + images[currentImage].folder:''}
               </span>
            </div>
        ]
    };

    const showGallery = ()=> {
        return (
            <Gallery ref={node=>{setComp(node);window.t = node}}
                     images={images}
                     showImageCount={false}
                     lightboxWillClose={()=>setLightboxVisible(false)}
                     lightboxWillOpen={()=>setLightboxVisible(true)}
                     onSelectImage={selectImage}
                     enableImageSelection={canAdmin===true}
                     currentImageWillChange={indexImage=>{
                         setCurrentImage(indexImage)
                         setImageToZoom(images[indexImage].hdLink);
                     }}
                     customControls={getCustomActions()}
                     showLightboxThumbnails={showThumbnails}
                     backdropClosesModal={true} lightboxWidth={2000}/>
        );
    };

    const isFolderEmpty = ()=>{
        return urlFolder !=="" && urlFolder.load !== "" && images.length === 0;
    };

    const showEmptyMessage = ()=> {
        return (
            urlFolder === '' || urlFolder.load === '' ? <></>:
                <Empty style={{marginTop:'40vh'}} description={<span style={{color:'white',fontWeight:'bold'}}>Pas de photos</span>} image={Empty.PRESENTED_IMAGE_SIMPLE} />
        );
    };

    const [scale,setScale] = useState(1);
    const [posX,setPosX] = useState(1);
    const [posY,setPosY] = useState(1);
    return (
        <>
            <Row className={"options"}>
                <Col flex={"200px"}>
                    {titleGallery}
                    {images.length} <PictureOutlined />
                </Col>
                <Col flex={"100px"}>
                    {showSelected()}
                </Col>
                <Col flex={"135px"}>
                    {showUpdateLink()}
                </Col>
                <Col flex={"auto"}>
                    {
                        tags
                            .sort((a,b)=>a.value < b.value ? -1:1)
                            .map(t=>
                                <Tooltip key={`tp${t.value}`} trigger={"click"} title={
                                    <CirclePicker width={'250px'} onChange={color=>updateColor(color,t)} circleSize={26} circleSpacing={8}/>
                                }><Tag key={t.value} color={t.color} closable={true} onClose={()=>removeTag(t)}>{t.value}</Tag>
                                </Tooltip>
                            )}
                    {!showInputTag && urlFolder.load !== ''?<Tag color="gray" onClick={()=>setShowInputTag(true)}><PlusOutlined /> tag</Tag>:<></>}
                    {showInputTag ? <Input size={"small"} style={{width:78+'px'}} onKeyUp={updateText} autoFocus={true} />:<></>}
                </Col>
            </Row>
            <Row className={"gallery"}>
                <Col span={24} style={{marginTop:36+'px'}}>
                    {images.length === 0 ? showEmptyMessage():showGallery()}
                </Col>
                <Modal visible={zoomEnable}
                       onCancel={()=>{
                           setScale(1);
                           setPosX(0);
                           setPosY(0);
                           setZoomEnable(false)}}
                       width={90+'%'}
                       style={{top:20}}
                       footer={[]}
                       closeIcon={<CloseOutlined style={{color:'white',fontSize:20}}/>}
                       wrapClassName={"modal-zoom"}>
                    <TransformWrapper scale={scale} positionX={posX} positionY={posY}>
                        <TransformComponent>
                            <img src={imageToZoom} alt="Zoomed panel"/>
                        </TransformComponent>
                    </TransformWrapper>
                </Modal>
                <SharePanel showSharePanel={showSharePanel} hide={()=>setShowSharePanel(false)} path={urlFolder.path} />
            </Row>
        </>
    )
}