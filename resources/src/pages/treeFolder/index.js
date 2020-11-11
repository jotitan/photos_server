import React, {useEffect, useState} from 'react'
import {Input, Tree} from 'antd'
import axios from "axios";
import useLocalStorage from "../../services/local-storage.hook";

export const getBaseUrlHref = ()=>getBaseUrl(window.location.href);

export const getBaseUrl = (defaultValue=window.location.origin)=>{
    switch (window.location.hostname) {
        case 'localhost':
            return 'http://localhost:9006';
        default : return defaultValue;
    }
}

const { Search } = Input;

const sortByName = (a,b)=>a.Name === b.name ? 0:a.Name < b.Name ? -1:1;

const adapt = node => {
    let data = {
        title:node.Name.replace(/_/g," "),
        key:getBaseUrl() + node.Link,
        path:node.Path,
        tags:getBaseUrl() + node.LinkTags,
        hasImages:node.HasImages
    };

    if(node.Children != null && node.Children.length > 0){
        data.children = node.Children.sort(sortByName).map(adapt);
    }else{
        data.isLeaf=true
    }
    return data;
};

export default function TreeFolder({setUrlFolder,setTitleGallery,update}) {
    const [tree,setTree] = useState([]);
    const [originalTree,setOriginalTree] = useState([]);
    const { DirectoryTree } = Tree;
    const [height,setHeight] = useState(window.innerHeight-185);
    const [expandables,setExpandables] = useLocalStorage("expandables",[])
    useEffect(()=>{
        axios({
            method:'GET',
            url:getBaseUrl() + '/rootFolders',
        }).then(d=>{
            if(d.data.Children != null) {
                let data = d.data.Children.map(adapt);
                setTree(data);
                setOriginalTree(data);
            }
        })
    },[update]);

    const hasName = (value,node,root,paths)=>{
        if(node.title.toLowerCase().indexOf(value) !== -1 || paths[root] != null){
            return node;
        }
        if(node.children == null || node.children.length === 0){
            // Leaf
            return null;
        }else{
            let children = node.children.map(a=>hasName(value,a,root + '/' + a.title,paths)).filter(a=>a!=null);
            if(children.length > 0){
                return {title:node.title,children:children,key:node.key,tags:node.tags,hasImages:node.hasImages};
            }
            return null;
        }
    };

    const filter = value => {
        // ask server
        axios({
            url:getBaseUrl() + `/filterTagsFolder?value=${value}`
        }).then(d=>{
            // Create map
            let map = {};
            d.data.forEach(d=>map[d.replace(/_/g," ")]=true);
            // Keep values from server and from name
            let values = originalTree.map(n=>hasName(value.toLowerCase(),n,n.title,map)).filter(n=>n!=null);
            setTree(values);
        })
    };

    const filterTree = event => {
        if(event.key === "Enter"){
            filter(event.target.value);
        }
    };

    const onSelect = (e,f)=>{
        setTitleGallery('');
        if(f.node.children == null || f.node.children.length === 0) {
            setUrlFolder({load:e[0],tags:f.node.tags,path:f.node.path})
        }else{
            // Case when folder has sub folders but also images
            if(f.node.hasImages){
                setUrlFolder({load:e[0],tags:f.node.tags,path:f.node.path})
            }
        }
    };

    window.addEventListener('resize', ()=>setHeight(window.innerHeight-185));


    return(
            <>
                <Search onKeyUp={filterTree} size={"small"} placeholder={"Filtrer par tag ou par nom"} style={{marginLeft:10+'px',width:280+'px',marginRight:10+'px'}}/>
                {tree.length > 0 ? <DirectoryTree
                    onSelect={onSelect}
                    treeData={tree}
                    height={height}
                    autoExpandParent={false}
                    expandedKeys={expandables}
                    onExpand={setExpandables}
                    virtual={true}
                    style={{
                        fontSize: 12 + 'px',
                        width: 300 + 'px',
                        overflow: 'auto',
                        backgroundColor: '#001529',
                        color: '#999'
                    }}
                />:''}
                </>
    )
}